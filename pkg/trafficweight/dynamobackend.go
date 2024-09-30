package trafficweight

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	awssession "github.com/adevinta/k8s-traffic-controller/pkg/aws"
)

type dynamodbBackend struct {
	Log         logr.Logger
	clusterName string
	awsRegion   string
	service     dynamodbiface.DynamoDBAPI
	tableName   string
}

func NewDynamodbBackend(logger logr.Logger, clusterName string, awsRegion string, tableName string) TrafficWeightBackend {
	logger = logger.WithValues("Backend", "dynamoDB")
	backend := dynamodbBackend{Log: logger, clusterName: clusterName, awsRegion: awsRegion, tableName: tableName}
	session, err := awssession.NewAwsSession(&awssession.SessionParameters{Region: backend.awsRegion, MaxRetries: 10})
	if err != nil {
		log.Fatalf("Error trying to create AWS session. %s", err)
	}

	backend.service = dynamodb.New(session)
	backend.initializeRowIfNotExist(Store)

	return &backend
}

type Item struct {
	ClusterName   string
	DesiredWeight int
	CurrentWeight int
	HealthCheckID string
}

type DynamoNoResultsError struct {
	msg string
}

func (e *DynamoNoResultsError) Error() string { return e.msg }

func (b *dynamodbBackend) ReadItem() (*Item, error) {
	result, err := b.service.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(b.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"ClusterName": {
				S: aws.String(b.clusterName),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Item) == 0 {
		return nil, &DynamoNoResultsError{"Item not found"}
	}

	item := Item{}

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		b.Log.Error(err, ". Failed to unmarshal Record")
		return nil, err
	}
	return &item, nil
}

func (b *dynamodbBackend) ReadWeight() (int, error) {
	item, err := b.ReadItem()
	if err != nil {
		return 0, err
	}

	return item.DesiredWeight, nil
}

func (b *dynamodbBackend) OnWeightUpdate(store StoreConfig) error {
	currentWeight := aws.String(fmt.Sprintf("%d", store.CurrentWeight))
	return b.write(&dynamodb.Update{
		TableName: aws.String(b.tableName),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":c": {
				N: currentWeight,
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			"ClusterName": &dynamodb.AttributeValue{
				S: aws.String(b.clusterName),
			},
		},
		UpdateExpression: aws.String("SET CurrentWeight = :c"),
	})
}

func (b *dynamodbBackend) initializeRowIfNotExist(store StoreConfig) {
	_, err := b.ReadWeight()
	if _, ok := err.(*DynamoNoResultsError); ok {
		b.Log.Info(fmt.Sprintf("Coudn't find previous configuration. Creating it..."))
		b.initializeClusterRow(Store)
	}
}

func (b *dynamodbBackend) initializeClusterRow(store StoreConfig) error {
	desiredWeight := aws.String(fmt.Sprintf("%d", store.DesiredWeight))
	currentWeight := aws.String(fmt.Sprintf("%d", store.CurrentWeight))

	return b.write(&dynamodb.Update{
		TableName: aws.String(b.tableName),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":d": {
				N: desiredWeight,
			},
			":c": {
				N: currentWeight,
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			"ClusterName": &dynamodb.AttributeValue{
				S: aws.String(b.clusterName),
			},
		},
		UpdateExpression: aws.String("SET DesiredWeight = :d, CurrentWeight = :c"),
	})
}

func (b *dynamodbBackend) write(update *dynamodb.Update) error {
	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: []*dynamodb.TransactWriteItem{
			&dynamodb.TransactWriteItem{
				Update: update,
			},
		},
	}

	_, err := b.service.TransactWriteItems(input)
	if err != nil {
		switch t := err.(type) {
		case *dynamodb.TableAlreadyExistsException:
			b.Log.Error(err, " There is an already on going transaction (more than one controller running?)")
		case *dynamodb.TransactionCanceledException:
			b.Log.Error(err, " failed to write items: %s\n%v", t.Message(), t.CancellationReasons)
		default:
			log.Fatalf("failed to write items: %v", err)
		}
	}
	return err
}
