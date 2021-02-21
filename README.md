# Wren Badge Rotator

[Read the article](https://medium.com/) that provides a deep dive of this repo and the problem it is solving. 

This is an AWS Lambda serverless function that is triggered by a CloudWatch event to run once per month. It updates my Wren.co badge on my Github profile with my latest stats 100% autonomously. 

# Cloudformation

This app is defined via Cloudformation in `template.yml` which creates: 
* The S3 bucket that will host the HTML page containing the modified badge 
* The S3 public access bucket policy allowing uploaded objects to be read by anonymous principals
* The AWS Lambda function that handles all the logic for: 
	* Fetching my current badge's raw HTML 
	* Translating its styling on the fly via Golang templates and modified CSS rules 
	* Writing the modified HTML to a page and publishing it via S3
	* Sending the request to the HCTI API to extract the image found in the HTML page 
	* Writing the extracted updated badge image locally and pushing it to S3 for safekeeping / debugging
	* Cloning my Github profile repository, updating its badge, and programmatically opening a Pull Request  
* The IAM Policy allowing the Lambda function to upload images to the S3 bucket 

# Pre-requisites 

[Install the AWS-SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html), and export your credentials (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`) to your shell via your preferred method. 

# Deployment 

`sam build && sam deploy --guided`

# Configure your Lambda env variables in the Web Console

After successfully deploying the stack to AWS, you'll need to go into the Lambda function that was created and set the following environment variables: 

* `GITHUB_OAUTH_TOKEN` - Your Github personal access token that has repo access scope
* `HCTI_API_USER_ID` - Your hcti.io User ID (create an account)
* `HCTI_API_KEY` - Your hcti.io API key 

Note that the `S3_BUCKET` and `WREN_USERNAME` env vars are also required by the Lambda function, but they are defined by the `template.yml`'s Lambda Environment property.

# N.B. 

If you wanted to use this yourself and run it - you'll need to make note of where I have environment variables defined (in the `template.yml` that are specific to my use-case). You'll want to update those to point at your own repo and your own Wren.co username
