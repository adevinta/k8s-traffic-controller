package trafficweight

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Define a mock struct to be used in your unit tests of myFunc.
type mockDynamoDBClient struct {
	dynamodbiface.DynamoDBAPI
	written       *dynamodb.TransactWriteItemsInput
	currentWeight *string
	desiredWeight *string
}

func (m *mockDynamoDBClient) GetItem(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	if m.written == nil {
		return &dynamodb.GetItemOutput{
			Item: map[string]*dynamodb.AttributeValue{},
		}, nil
	}
	return &dynamodb.GetItemOutput{
		Item: map[string]*dynamodb.AttributeValue{
			"ClusterName": &dynamodb.AttributeValue{
				S: aws.String("lolo"),
			},
			"DesiredWeight": &dynamodb.AttributeValue{
				N: m.desiredWeight,
			},
			"CurrentWeight": &dynamodb.AttributeValue{
				N: m.currentWeight,
			},
			"foo": &dynamodb.AttributeValue{
				N: m.currentWeight,
			},
		},
	}, nil
}

func (m *mockDynamoDBClient) TransactWriteItems(input *dynamodb.TransactWriteItemsInput) (*dynamodb.TransactWriteItemsOutput, error) {
	m.written = input
	if v, found := input.TransactItems[0].Update.ExpressionAttributeValues[":c"]; found {
		m.currentWeight = v.N
	}
	if v, found := input.TransactItems[0].Update.ExpressionAttributeValues[":d"]; found {
		m.desiredWeight = v.N
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
}

func TestOnWeightUpdate(t *testing.T) {
	mockSvc := &mockDynamoDBClient{}

	dynamoBackend := dynamodbBackend{
		service: mockSvc,
	}
	Store.DesiredWeight = 10

	assert.Equal(t, dynamoBackend.OnWeightUpdate(StoreConfig{CurrentWeight: 35, DesiredWeight: Store.DesiredWeight}), nil)
	assert.Equal(t, mockSvc.written.TransactItems[0].Update.ExpressionAttributeValues, map[string]*dynamodb.AttributeValue{
		":c": {
			N: aws.String("35"),
		},
	})
	assert.Equal(t, *mockSvc.written.TransactItems[0].Update.UpdateExpression, "SET CurrentWeight = :c")
}

func TestInitializeClusterRow(t *testing.T) {
	mockSvc := &mockDynamoDBClient{}

	dynamoBackend := dynamodbBackend{
		service: mockSvc,
	}
	Store.DesiredWeight = 10

	assert.Equal(t, dynamoBackend.initializeClusterRow(StoreConfig{CurrentWeight: 35, DesiredWeight: Store.DesiredWeight}), nil)
	assert.Equal(t, mockSvc.written.TransactItems[0].Update.ExpressionAttributeValues, map[string]*dynamodb.AttributeValue{
		":d": {
			N: aws.String("10"),
		},
		":c": {
			N: aws.String("35"),
		},
	})
	assert.Equal(t, *mockSvc.written.TransactItems[0].Update.UpdateExpression, "SET DesiredWeight = :d, CurrentWeight = :c")

	w, e := dynamoBackend.ReadWeight()
	assert.Equal(t, Store.DesiredWeight, w)
	assert.Nil(t, e)
}

func TestInitializeRowIfNotExists(t *testing.T) {
	mockSvc := &mockDynamoDBClient{}

	dynamoBackend := dynamodbBackend{
		service: mockSvc,
		Log:     zap.New(zap.UseDevMode(true)),
	}

	w, e := dynamoBackend.ReadWeight()
	assert.Equal(t, 0, w)
	assert.NotNil(t, e)

	Store.DesiredWeight = 50
	dynamoBackend.initializeRowIfNotExist(StoreConfig{CurrentWeight: 35, DesiredWeight: Store.DesiredWeight})
	w, e = dynamoBackend.ReadWeight()
	assert.Equal(t, Store.DesiredWeight, w)
	assert.Nil(t, e)

	Store.DesiredWeight = 100
	dynamoBackend.initializeRowIfNotExist(StoreConfig{CurrentWeight: 35, DesiredWeight: Store.DesiredWeight})
	w, e = dynamoBackend.ReadWeight()
	assert.Equal(t, 50, w)
	assert.Nil(t, e)
}
