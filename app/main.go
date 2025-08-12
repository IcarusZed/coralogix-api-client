package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	gen "github.com/IcarusZed/coralogix-api-client/generated"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

const (
	// these can be in vars/config file
	CORALOGIX_API_URL     = "https://api.eu2.coralogix.com/mgmt/openapi"
	WEBHOOK_URL           = "https://bptfmrfiz7.execute-api.eu-north-1.amazonaws.com/default/fn-webhook"
	CORALOGIX_API_TIMEOUT = 10 * time.Second
	ERROR_NUMBER          = "50"
	APPLICATION_NAME      = "sample-app"
	SUBSYSTEM_NAME        = "yak"
)

var CORALOGIX_API_KEY string

func getLuceneQueryForErrorNumber(errorNumber string) string {
	return fmt.Sprintf("logRecord.body:\"error\\: %s\"", errorNumber)
}

func createOutgoingWebhook(client *gen.ClientWithResponses, webhookURL string) (string, error) {
	resp, err := client.OutgoingWebhooksServiceCreateOutgoingWebhook(context.Background(), gen.V1CreateOutgoingWebhookRequest{
		Data: gen.V1OutgoingWebhookInputData{
			Name: "AWS fn webhook",
			Type: gen.V1WebhookTypeGENERIC,
			Url:  &webhookURL,
			GenericWebhook: &gen.V1GenericWebhookConfig{
				Method: gen.GenericWebhookConfigMethodTypeGET,
				Uuid:   uuid.NewString(),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("error calling CreateOutgoingWebhook: %w", err)
	}
	responseObj, err := gen.ParseOutgoingWebhooksServiceCreateOutgoingWebhookResponse(resp)
	if err != nil {
		return "", fmt.Errorf("error parsing CreateOutgoingWebhook response: %w", err)
	}
	if responseObj == nil || responseObj.JSON200 == nil {
		return "", fmt.Errorf("CreateOutgoingWebhook response object is nil or JSON200 is nil")
	}

	return responseObj.JSON200.Id, nil
}

func getWebhookExternalID(client *gen.ClientWithResponses, webhookID string) (int64, error) {
	resp, err := client.OutgoingWebhooksServiceGetOutgoingWebhook(context.Background(), webhookID)
	if err != nil {
		return -1, fmt.Errorf("error calling GetOutgoingWebhook: %w", err)
	}
	responseObj, err := gen.ParseOutgoingWebhooksServiceGetOutgoingWebhookResponse(resp)
	if err != nil {
		return -1, fmt.Errorf("error parsing GetOutgoingWebhook response: %w", err)
	}
	if responseObj == nil || responseObj.JSON200 == nil {
		return -1, fmt.Errorf("GetOutgoingWebhook response object is nil or JSON200 is nil")
	}
	return responseObj.JSON200.Webhook.ExternalId, nil
}

func createAlertDef(client *gen.ClientWithResponses, webhookExternalId int64, name string, description string, filterLuceneQuery string) (string, string, error) {
	timeWindow30Minutes := gen.LOGSTIMEWINDOWVALUEMINUTES30

	resp, err := client.AlertDefsServiceCreateAlertDef(context.Background(), gen.V3AlertDefProperties{
		Name:        fmt.Sprintf("Alert for increased error number %s occurence", ERROR_NUMBER),
		Description: &description,
		Type:        gen.V3AlertDefTypeALERTDEFTYPELOGSTHRESHOLD,
		Priority:    gen.V3AlertDefPriorityALERTDEFPRIORITYP1,
		NotificationGroup: &gen.V3AlertDefNotificationGroup{
			Webhooks: &[]gen.V3AlertDefWebhooksSettings{
				{
					Integration: gen.Alertsv3IntegrationType{
						IntegrationId: &webhookExternalId,
					},
				},
			},
		},
		LogsThreshold: &gen.V3LogsThresholdType{
			Rules: []gen.V3LogsThresholdRule{
				{
					Condition: gen.V3LogsThresholdCondition{
						ConditionType: gen.LOGSTHRESHOLDCONDITIONTYPEMORETHANORUNSPECIFIED,
						Threshold:     2,
						TimeWindow: gen.V3LogsTimeWindow{
							LogsTimeWindowSpecificValue: &timeWindow30Minutes,
						},
					},
				},
			},
			LogsFilter: &gen.Alertsv3LogsFilter{
				SimpleFilter: &gen.V3LogsSimpleFilter{
					LuceneQuery: &filterLuceneQuery,
					// Redundant with current existing logs as 'error: <error_number>' only exists with these labels
					LabelFilters: &gen.V3LabelFilters{
						ApplicationName: &[]gen.V3LabelFilterType{
							{
								Operation: gen.LOGFILTEROPERATIONTYPEISORUNSPECIFIED,
								Value:     APPLICATION_NAME,
							},
						},
						SubsystemName: &[]gen.V3LabelFilterType{
							{
								Operation: gen.LOGFILTEROPERATIONTYPEISORUNSPECIFIED,
								Value:     SUBSYSTEM_NAME,
							},
						},
						Severities: &[]gen.V3LogSeverity{
							gen.LOGSEVERITYERROR,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("error calling CreateAlertDef: %w", err)
	}

	responseObj, err := gen.ParseAlertDefsServiceCreateAlertDefResponse(resp)
	if err != nil {
		return "", "", fmt.Errorf("error parsing CreateAlertDef response: %w", err)
	}

	if responseObj == nil || responseObj.JSON200 == nil {
		return "", "", fmt.Errorf("response object is nil or JSON200 is nil")
	}

	return responseObj.JSON200.AlertDef.Id, responseObj.JSON200.AlertDef.AlertVersionId, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
		return
	}

	CORALOGIX_API_KEY = os.Getenv("CORALOGIX_API_KEY")
	if CORALOGIX_API_KEY == "" {
		fmt.Println("CORALOGIX_API_KEY missing or empty in .env file")
		return
	}

	httpClient := http.Client{
		Timeout: CORALOGIX_API_TIMEOUT,
	}

	client, err := gen.NewClientWithResponses(CORALOGIX_API_URL, gen.WithHTTPClient(&httpClient), gen.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", CORALOGIX_API_KEY)
		return nil
	}))

	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	webhookId, err := createOutgoingWebhook(client, WEBHOOK_URL)
	if err != nil {
		fmt.Printf("Error creating outgoing webhook: %v\n", err)
		return
	}
	fmt.Printf("Outgoing webhook created successfully with ID: %s\n", webhookId)
	webhookExternalId, err := getWebhookExternalID(client, webhookId)
	if err != nil {
		fmt.Printf("Error getting webhook external ID: %v\n", err)
		return
	}
	fmt.Printf("Webhook external ID: %d\n", webhookExternalId)

	alertId, alertVersionId, err := createAlertDef(client, webhookExternalId, "Increased Error Number Alert", "This alert triggers when the error number exceeds a threshold.", getLuceneQueryForErrorNumber(ERROR_NUMBER))
	if err != nil {
		fmt.Printf("Error creating alert definition: %v\n", err)
		return
	}

	fmt.Printf("Alert created successfully with ID: %s, Version ID: %s\n", alertId, alertVersionId)
}
