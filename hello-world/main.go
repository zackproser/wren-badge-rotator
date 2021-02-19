package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	WrenBadgeURL = "https://www.wren.co/badge/logo/zackproser"

	// ErrNon200Response non 200 status code in response
	ErrNon200Response = errors.New("Non 200 Response found")
)

// Badge recursively searches the HTML document to find the badge node, which is wrapped in an "a" tag, or link
func Badge(doc *html.Node) (*html.Node, error) {
	var badge *html.Node
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			badge = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(doc)
	if badge != nil {
		return badge, nil
	}
	return nil, errors.New("Missing <a> in the node tree")
}

// renderNode converts the supplied html node to a string
func renderNode(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	resp, err := http.Get(WrenBadgeURL)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	if resp.StatusCode != 200 {
		return events.APIGatewayProxyResponse{}, ErrNon200Response
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	fmt.Printf("Got response body: %s\n", string(b))

	doc, _ := html.Parse(strings.NewReader(string(b)))
	bn, err := Badge(doc)

	fmt.Printf("Got badge node: %+v\n", bn)

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

	fmt.Printf("Got rendered badge html: %+v", badge)

	// Remove subscript 2 that can't be rendered directly
	badge.Contents = strings.Replace(badge.Contents, "₂", "2", -1)

	t := template.Must(template.New("wrapper").Parse(wrapper))

	templateErr := t.Execute(os.Stdout, badge)
	if templateErr != nil {
		fmt.Printf("Error rendering template: %+v\n", templateErr)
	}

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
