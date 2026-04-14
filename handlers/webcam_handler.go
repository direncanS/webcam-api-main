package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	st "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/aws/smithy-go"
	"github.com/lambda-lama/webcam-api/db"
)

type WebcamData struct {
	Image     string            `json:"image"`
	Topic     string            `json:"topic"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
}

type SQSSendMessageAPI interface {
	GetQueueUrl(ctx context.Context,
		params *sqs.GetQueueUrlInput,
		optFns ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)

	SendMessage(ctx context.Context,
		params *sqs.SendMessageInput,
		optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

func GetQueueURL(c context.Context, api SQSSendMessageAPI, input *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
	return api.GetQueueUrl(c, input)
}

func SendMsg(c context.Context, api SQSSendMessageAPI, input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	return api.SendMessage(c, input)
}

func uploadImageS3(bucketName string, bucketKey string, base64Image string) error {
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		fmt.Println("Error while loading the aws config ", err)
		return err
	}

	client := s3.NewFromConfig(config)
	fmt.Println("Created client")

	imageBytes, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		fmt.Println("Error decoding image")
		return err
	}

	var output *s3.ListBucketsOutput
	var buckets []st.Bucket
	output, err = client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "AccessDenied" {
			fmt.Println("You don't have permission to list buckets for this account.")
			err = apiErr
		} else {
			log.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
		}
		return err
	}
	buckets = output.Buckets
	fmt.Println("Available buckets:")
	for _, bucket := range buckets {
		fmt.Printf("\t%v\n", *bucket.Name)
	}

	fmt.Println("Decoded Image")
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(bucketKey),
		Body:        bytes.NewReader(imageBytes),
		ContentType: aws.String("image/jpeg"),
	})

	fmt.Println("AFTER PUT OBJECT")
	if err != nil {
		fmt.Println("Error uploading file: ", err)
		return err
	}

	return nil
}

func triggerQueue(imageObjectKey string, bucketName string) error {
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		fmt.Println("Error while loading the aws config ", err)
		return err
	}
	client := sqs.NewFromConfig(config)

	queue := "serverless-sqs" // TODO: do this with env vars
	gQInput := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queue),
	}

	fmt.Println("Before GetQueueURL")
	result, err := GetQueueURL(context.TODO(), client, gQInput)
	fmt.Printf("After GetQueueURL %s\n", *result.QueueUrl)
	if err != nil {
		fmt.Println("Got an error getting the queue URL:")
		fmt.Println(err)
		return err
	}

	queueURL := result.QueueUrl

	fmt.Println("SendMessageInput")
	sendMessageInput := &sqs.SendMessageInput{
		DelaySeconds: 10,
		MessageAttributes: map[string]types.MessageAttributeValue{
			"image_object_key": {
				DataType:    aws.String("String"),
				StringValue: aws.String(imageObjectKey),
			},
			"bucket_name": {
				DataType:    aws.String("String"),
				StringValue: aws.String(bucketName),
			},
		},
		MessageBody: aws.String(imageObjectKey),
		QueueUrl:    queueURL,
	}

	resp, err := SendMsg(context.TODO(), client, sendMessageInput)
	if err != nil {
		fmt.Println("Got an error sending the message:")
		fmt.Println(err)
		return err
	}

	fmt.Println("Sent message with ID: " + *resp.MessageId)

	return nil
}

func insertWebcamDataIntoPostgres(imageObjectKey string, topic string, metadata map[string]string, createdAt time.Time) error {
	conn, err := db.GetConnection()
	if err != nil {
		fmt.Print("Unable to connect to DB")
		return errors.New("unable to connect to DB")
	}
	defer conn.Close(context.Background())
	fmt.Printf("Connection available: %s", conn.Config().Host)

	// Create a new INSERT statement
	var insertStmt string
	if createdAt.IsZero() {
		insertStmt = `INSERT INTO webcam_data (image_object_key, topic, metadata) VALUES ($1, $2, $3)`
		_, err = conn.Exec(context.Background(), insertStmt, imageObjectKey, topic, metadata)
		if err != nil {
			fmt.Println("Unable to insert ", err)
			return err
		}
	} else {
		insertStmt = `INSERT INTO webcam_data (image_object_key, topic, metadata, created_at) VALUES ($1, $2, $3, $4)`
		_, err = conn.Exec(context.Background(), insertStmt, imageObjectKey, topic, metadata, createdAt)
		if err != nil {
			fmt.Println("Unable to insert ", err)
			return err
		}
	}

	return nil
}

func WebcamCreate(w http.ResponseWriter, r *http.Request) {
	var webcamData WebcamData

	err := json.NewDecoder(r.Body).Decode(&webcamData)
	if err != nil {
		fmt.Println("Error parsing json ", err)
		sendError(w, http.StatusBadRequest, "Error parsing json")
		return
	}

	fmt.Println("webcam metadata: ", webcamData.Metadata)
	err = validateWebcamData(webcamData)
	if err != nil {
		fmt.Println("Fields cannot be empty")
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	imageObjectKey := fmt.Sprintf("image-%s.jpg", time.Now().UTC().Format("20020102150405"))

	err = uploadImageS3(os.Getenv("S3_BUCKET"), imageObjectKey, webcamData.Image)
	if err != nil {
		fmt.Printf("Some error: %s", err.Error())
		sendError(w, http.StatusInternalServerError, "Failed to upload to S3")
		return
	}

	fmt.Println("Before db insert")
	err = insertWebcamDataIntoPostgres(imageObjectKey, webcamData.Topic, webcamData.Metadata, webcamData.CreatedAt)
	if err != nil {
		fmt.Printf("DB error: %s", err.Error())
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	fmt.Println("After db insert")

	err = triggerQueue(imageObjectKey, os.Getenv("S3_BUCKET"))
	if err != nil {
		fmt.Printf("SQS error: %s", err.Error())
		sendError(w, http.StatusInternalServerError, "Internal Server Error")
	}

	sendResponse(w, http.StatusCreated, "CREATED")
}

func validateWebcamData(webcamData WebcamData) error {
	// Check if the image field is empty
	if webcamData.Image == "" {
		return errors.New("image field is required")
	}

	// Check if the topic field is empty
	if webcamData.Topic == "" {
		return errors.New("topic field is required")
	}

	// Check if the metadata field is empty
	if len(webcamData.Metadata) == 0 {
		return errors.New("metadata field is required")
	}

	return nil
}
