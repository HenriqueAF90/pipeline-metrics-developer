package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/sirupsen/logrus"

	"aggregator/internal/usecase"
)

// Handler agrupa os handlers HTTP da API REST.
type Handler struct {
	queryUC   *usecase.QueryMetricsUseCase
	sqsClient *sqs.Client
	dbClient  *dynamodb.Client
	queueURL  string
	tableName string
	log       *logrus.Logger
}

func NewHandler(
	queryUC *usecase.QueryMetricsUseCase,
	sqsClient *sqs.Client,
	dbClient *dynamodb.Client,
	queueURL string,
	tableNameEvents string,
	log *logrus.Logger,
) *Handler {
	return &Handler{
		queryUC:   queryUC,
		sqsClient: sqsClient,
		dbClient:  dbClient,
		queueURL:  queueURL,
		tableName: tableNameEvents,
		log:       log,
	}
}

// Register registra todas as rotas no mux fornecido.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.healthHandler)
	mux.HandleFunc("/metrics/", h.metricsRouter)
}

func (h *Handler) metricsRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/metrics/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	developerID := parts[0]

	switch {
	case len(parts) == 2 && parts[1] == "summary":
		h.summaryHandler(w, r, developerID)
	case len(parts) == 1:
		h.eventsHandler(w, r, developerID)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) summaryHandler(w http.ResponseWriter, r *http.Request, developerID string) {
	summary, err := h.queryUC.GetSummary(r.Context(), developerID)
	if err != nil {
		h.log.WithError(err).Error("failed to get summary")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) eventsHandler(w http.ResponseWriter, r *http.Request, developerID string) {
	events, err := h.queryUC.GetEvents(r.Context(), developerID)
	if err != nil {
		h.log.WithError(err).Error("failed to get events")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	details := map[string]string{"sqs": "connected", "dynamodb": "connected"}
	status := "ok"
	httpCode := http.StatusOK

	if _, err := h.sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: &h.queueURL,
	}); err != nil {
		status = "degraded"
		details["sqs"] = fmt.Sprintf("error: %v", err)
		httpCode = http.StatusServiceUnavailable
	}

	if _, err := h.dbClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: &h.tableName,
	}); err != nil {
		status = "degraded"
		details["dynamodb"] = fmt.Sprintf("error: %v", err)
		httpCode = http.StatusServiceUnavailable
	}

	writeJSON(w, httpCode, map[string]interface{}{
		"status":  status,
		"details": details,
	})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Nada a fazer após WriteHeader já ser chamado
	}
}
