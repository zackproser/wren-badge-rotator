package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"golang.org/x/net/html"
)

var (
	// WrenBadgeURL is the page where Wren hosts my original badge, that will need to be captured, modified and re-hosted prior to extraction
	WrenBadgeURL = fmt.Sprintf("https://www.wren.co/badge/logo/%s", os.Getenv("WREN_USERNAME"))
	// S3_REGION is one of the environment varibles automatically injected by the Lambda execution runtime
	S3_REGION = os.Getenv("AWS_REGION")
	// S3_BUCKET is an environment variable determined and injected by the Cloudformation that creates the project bucket and its bucket access policy
	S3_BUCKET = os.Getenv("S3_BUCKET")
	// BADGE_LOCAL_PATH is the path in the lambda execution environment where we will temporarily write the HTML page containing our modifified CSS
	BADGE_LOCAL_PATH = "/tmp/badge.html"
	// HTML_PAGE_DEST_S3_PATH is the path in S3 where the modified badge HTML page will be written
	HTML_PAGE_DEST_S3_PATH = "badge.html"
	// EXTRACTED_BADGE_IMAGE_LOCAL_PATH is the path where the image returned by the HTCI API will be written, prior to be uploaded to S3
	EXTRACTED_BADGE_IMAGE_LOCAL_PATH = "/tmp/extracted-badge.png"
	// EXTRACTED_BADGE_IMAGE_S3_PATH is the path in S3 where the updated and extracted badge image will be written for debugging and testing purposes (it is not used directly)
	EXTRACTED_BADGE_IMAGE_S3_PATH = "/extracted/badge.png"
	// S3_BADGE_PATH is the S3 bucket address to the public HTML page - that can be provided to the HCTI API for image extraction
	S3_BADGE_HTML_PUBLIC_URL = fmt.Sprintf("https://%s.s3.amazonaws.com/badge.html", S3_BUCKET)
	// HCTI_API_URL is the URL to the API that converts HTML and CSS to a static image
	HCTI_API_URL = "https://hcti.io/v1/image"
	// REPO_URL points to the special repo that stylizes my Github profile
	REPO_URL = "https://github.com/zackproser/zackproser.git"
)

// handler is the entrypoint called by Lambda when it is triggered by our CloudWatch event or a manual test or invocation
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	err := sanityCheckEnvVars()
	if err != nil {
		return events.APIGatewayProxyResponse{
				Body:       "You must define HCTI_API_KEY, HCTI_USER_NAME, and GITHUB_OAUTH_TOKEN env vars",
				StatusCode: 400,
			},
			nil
	}

	// Fetch the raw HTML of the page that hosts my Wren.co badge
	resp, err := http.Get(WrenBadgeURL)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	if resp.StatusCode != 200 {
		return events.APIGatewayProxyResponse{
			Body:       "Received non 200 status code response when fetching Wren badge",
			StatusCode: resp.StatusCode,
		}, nil
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	// Convert the raw bytes of the HTTP response to a string, and feed that string into the HTML parse function
	// so that we're left with an HTML node entity that can be passed into our badge function
	doc, _ := html.Parse(strings.NewReader(string(b)))
	// Badge recursively searches through the HTML nodes of the document to find just the <a> tag wrapping the entire badge
	bn, err := Badge(doc)

	if err != nil {
		return events.APIGatewayProxyResponse{
				Body:       "Error finding badge in document",
				StatusCode: 200,
			},
			nil
	}

	badge := BadgeHTML{
		Contents: renderNode(bn),
	}

	// Remove subscript 2 that can't be rendered directly
	badge.Contents = strings.Replace(badge.Contents, "₂", "2", -1)

	// Parse our modified HTML page template, that contains our own inline CSS rules that will contstrain the height and width
	// of our badge so that it does not spill across the full-width of the viewport
	t := template.Must(template.New("wrapper").Parse(wrapper))

	// For debugging purposes, write the rendered content of our HTML page template to STDOUT so we can view it in the logs and sanity-check it
	templateErr := t.Execute(os.Stdout, badge)
	if templateErr != nil {
		fmt.Printf("Error rendering template: %+v\n", templateErr)
	}

	// Now, write the modified HTML to a local file which will ultimately be uploaded to S3 as the modifed HTML page
	// that HCTI will be able to scrape to extract our badge image
	bfh, openErr := os.Create(BADGE_LOCAL_PATH)
	if openErr != nil {
		fmt.Printf("Error opening badge.png for writing: %+v\n", openErr)
	}

	writeErr := t.Execute(bfh, badge)
	if writeErr != nil {
		return events.APIGatewayProxyResponse{
				Body:       fmt.Sprintf("Error writing badge image file: %s", writeErr),
				StatusCode: 500,
			},
			nil
	}

	// Upload the modified HTML file containing the re-styled badge to S3
	uploadErr := uploadHTMLBadgeToS3()
	if uploadErr != nil {
		fmt.Printf("Error uploading modified HTML file to S3 bucket: %+b\n", err)
		return events.APIGatewayProxyResponse{
				Body:       fmt.Sprintf("Error uploading badge.html to S3: %+v\n", uploadErr),
				StatusCode: 500,
			},
			nil
	}

	// Call the HCTI Image processing API to convert the modified and published HTML document to a cropped badge img
	resizedImageURL, resizeErr := resizePostedBadge()
	if resizeErr != nil {
		fmt.Printf("Error calling HCTI API to convert HTML to image: %+v\n", resizeErr)
		return events.APIGatewayProxyResponse{
				Body:       fmt.Sprintf("Error calling HCTI API: %+v\n", resizeErr),
				StatusCode: 500,
			},
			nil
	}

	// Download the extracted badge from the URL that HCTI is hosting it at, and upload it to the S3 bucket
	copyErr := copyExtractedBadgeImageToS3(resizedImageURL)
	if copyErr != nil {
		fmt.Printf("Error copying extracted badge image from HCTI to S3 bucket: %+v\n", copyErr)
		return events.APIGatewayProxyResponse{
				Body:       fmt.Sprintf("Error copying image from HCTI: %+v\n", copyErr),
				StatusCode: 500,
			},
			nil
	}

	// Clone my special Github profile repository and overwrite the badge image it contains with the newly updated
	// badge image that was returned by the HCTI API and written locally to the lambda execution context
	updateErr := updateBadgeImage()
	if updateErr != nil {
		fmt.Printf("Error updating wren badge via git: %+v\n", updateErr)
		return events.APIGatewayProxyResponse{
				Body:       fmt.Sprintf("Error updating wren badge: %+v\n", updateErr),
				StatusCode: 500,
			},
			nil
	}

	// At this point, all processing steps have completed successfully, without error, so return a success response
	return events.APIGatewayProxyResponse{
			Body:       "Finished processing without error",
			StatusCode: 200,
		},
		nil
}

func main() {
	lambda.Start(handler)
}
