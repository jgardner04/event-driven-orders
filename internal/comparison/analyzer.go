package comparison

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

type DataAnalyzer struct {
	logger *logrus.Logger
}

type ComparisonResult struct {
	OrderServiceData []models.Order            `json:"order_service_data"`
	SAPData          []models.Order            `json:"sap_data"`
	Analysis         DetailedAnalysis          `json:"analysis"`
	Inconsistencies  []DataInconsistency       `json:"inconsistencies"`
	Statistics       ComparisonStatistics      `json:"statistics"`
	Recommendations  []string                  `json:"recommendations"`
	Timestamp        time.Time                 `json:"timestamp"`
}

type DetailedAnalysis struct {
	TotalOrderService     int                    `json:"total_order_service"`
	TotalSAP             int                    `json:"total_sap"`
	PerfectMatches       int                    `json:"perfect_matches"`
	PartialMatches       int                    `json:"partial_matches"`
	MissingInSAP         []string               `json:"missing_in_sap"`
	MissingInOrderService []string              `json:"missing_in_order_service"`
	DataMismatches       []DataMismatch         `json:"data_mismatches"`
	SyncPercentage       float64                `json:"sync_percentage"`
	OverallStatus        string                 `json:"overall_status"`
}

type DataInconsistency struct {
	OrderID      string                 `json:"order_id"`
	Type         string                 `json:"type"`
	Severity     string                 `json:"severity"`
	Field        string                 `json:"field,omitempty"`
	OSValue      interface{}            `json:"order_service_value,omitempty"`
	SAPValue     interface{}            `json:"sap_value,omitempty"`
	Description  string                 `json:"description"`
	Impact       string                 `json:"impact"`
	Suggestion   string                 `json:"suggestion"`
}

type DataMismatch struct {
	OrderID     string                 `json:"order_id"`
	Field       string                 `json:"field"`
	OSValue     interface{}            `json:"order_service_value"`
	SAPValue    interface{}            `json:"sap_value"`
	Difference  interface{}            `json:"difference,omitempty"`
}

type ComparisonStatistics struct {
	DataConsistencyScore  float64                `json:"data_consistency_score"`
	CriticalIssues       int                    `json:"critical_issues"`
	WarningIssues        int                    `json:"warning_issues"`
	InfoIssues           int                    `json:"info_issues"`
	FieldAccuracy        map[string]float64     `json:"field_accuracy"`
	AverageProcessingTime time.Duration         `json:"average_processing_time"`
	LastSyncTime         *time.Time             `json:"last_sync_time,omitempty"`
}

func NewDataAnalyzer(logger *logrus.Logger) *DataAnalyzer {
	return &DataAnalyzer{
		logger: logger,
	}
}

func (da *DataAnalyzer) CompareData(osOrders, sapOrders []models.Order) *ComparisonResult {
	startTime := time.Now()
	
	da.logger.WithFields(logrus.Fields{
		"order_service_count": len(osOrders),
		"sap_count":          len(sapOrders),
	}).Info("Starting comprehensive data comparison")

	result := &ComparisonResult{
		OrderServiceData: osOrders,
		SAPData:         sapOrders,
		Inconsistencies: []DataInconsistency{},
		Timestamp:       time.Now(),
	}

	// Create lookup maps
	osMap := make(map[string]*models.Order)
	sapMap := make(map[string]*models.Order)
	
	for i := range osOrders {
		osMap[osOrders[i].ID] = &osOrders[i]
	}
	
	for i := range sapOrders {
		sapMap[sapOrders[i].ID] = &sapOrders[i]
	}

	// Perform detailed analysis
	result.Analysis = da.performDetailedAnalysis(osMap, sapMap)
	result.Inconsistencies = da.findInconsistencies(osMap, sapMap)
	result.Statistics = da.calculateStatistics(result, time.Since(startTime))
	result.Recommendations = da.generateRecommendations(result)

	da.logger.WithFields(logrus.Fields{
		"processing_time":     time.Since(startTime),
		"inconsistencies":     len(result.Inconsistencies),
		"consistency_score":   result.Statistics.DataConsistencyScore,
	}).Info("Data comparison completed")

	return result
}

func (da *DataAnalyzer) performDetailedAnalysis(osMap, sapMap map[string]*models.Order) DetailedAnalysis {
	analysis := DetailedAnalysis{
		TotalOrderService:     len(osMap),
		TotalSAP:             len(sapMap),
		MissingInSAP:         []string{},
		MissingInOrderService: []string{},
		DataMismatches:       []DataMismatch{},
	}

	allOrderIDs := make(map[string]bool)
	for id := range osMap {
		allOrderIDs[id] = true
	}
	for id := range sapMap {
		allOrderIDs[id] = true
	}

	for orderID := range allOrderIDs {
		osOrder, osExists := osMap[orderID]
		sapOrder, sapExists := sapMap[orderID]

		if !osExists {
			analysis.MissingInOrderService = append(analysis.MissingInOrderService, orderID)
			continue
		}

		if !sapExists {
			analysis.MissingInSAP = append(analysis.MissingInSAP, orderID)
			continue
		}

		// Compare order details
		if da.isExactMatch(osOrder, sapOrder) {
			analysis.PerfectMatches++
		} else {
			analysis.PartialMatches++
			mismatches := da.compareOrderFields(osOrder, sapOrder)
			analysis.DataMismatches = append(analysis.DataMismatches, mismatches...)
		}
	}

	// Calculate sync percentage
	totalComparable := len(allOrderIDs)
	if totalComparable > 0 {
		syncedCount := analysis.PerfectMatches
		analysis.SyncPercentage = float64(syncedCount) / float64(totalComparable) * 100
	}

	// Determine overall status
	if analysis.SyncPercentage >= 95 {
		analysis.OverallStatus = "excellent"
	} else if analysis.SyncPercentage >= 85 {
		analysis.OverallStatus = "good"
	} else if analysis.SyncPercentage >= 70 {
		analysis.OverallStatus = "fair"
	} else {
		analysis.OverallStatus = "poor"
	}

	return analysis
}

func (da *DataAnalyzer) findInconsistencies(osMap, sapMap map[string]*models.Order) []DataInconsistency {
	var inconsistencies []DataInconsistency

	// Find missing orders
	for orderID := range osMap {
		if _, exists := sapMap[orderID]; !exists {
			inconsistencies = append(inconsistencies, DataInconsistency{
				OrderID:     orderID,
				Type:        "missing_in_sap",
				Severity:    "critical",
				Description: "Order exists in Order Service but missing in SAP",
				Impact:      "Customer orders may not be processed by legacy system",
				Suggestion:  "Run data migration to sync missing orders to SAP",
			})
		}
	}

	for orderID := range sapMap {
		if _, exists := osMap[orderID]; !exists {
			inconsistencies = append(inconsistencies, DataInconsistency{
				OrderID:     orderID,
				Type:        "missing_in_order_service",
				Severity:    "warning",
				Description: "Order exists in SAP but missing in Order Service",
				Impact:      "Historical data may be incomplete in new system",
				Suggestion:  "Consider migrating historical SAP data to Order Service",
			})
		}
	}

	// Find field mismatches
	for orderID, osOrder := range osMap {
		if sapOrder, exists := sapMap[orderID]; exists {
			fieldInconsistencies := da.compareFields(osOrder, sapOrder)
			inconsistencies = append(inconsistencies, fieldInconsistencies...)
		}
	}

	return inconsistencies
}

func (da *DataAnalyzer) compareFields(osOrder, sapOrder *models.Order) []DataInconsistency {
	var inconsistencies []DataInconsistency

	// Compare customer ID
	if osOrder.CustomerID != sapOrder.CustomerID {
		inconsistencies = append(inconsistencies, DataInconsistency{
			OrderID:     osOrder.ID,
			Type:        "field_mismatch",
			Severity:    "critical",
			Field:       "customer_id",
			OSValue:     osOrder.CustomerID,
			SAPValue:    sapOrder.CustomerID,
			Description: "Customer ID mismatch between systems",
			Impact:      "Order attribution error, customer experience issues",
			Suggestion:  "Investigate data transformation logic",
		})
	}

	// Compare total amount
	if math.Abs(osOrder.TotalAmount-sapOrder.TotalAmount) > 0.01 {
		inconsistencies = append(inconsistencies, DataInconsistency{
			OrderID:     osOrder.ID,
			Type:        "field_mismatch",
			Severity:    "critical",
			Field:       "total_amount",
			OSValue:     osOrder.TotalAmount,
			SAPValue:    sapOrder.TotalAmount,
			Description: "Total amount mismatch between systems",
			Impact:      "Financial discrepancy, billing issues",
			Suggestion:  "Review calculation logic and currency handling",
		})
	}

	// Compare status
	if osOrder.Status != sapOrder.Status {
		severity := "warning"
		if osOrder.Status == "pending" && sapOrder.Status == "confirmed" {
			severity = "info" // Expected during normal processing
		}
		
		inconsistencies = append(inconsistencies, DataInconsistency{
			OrderID:     osOrder.ID,
			Type:        "field_mismatch",
			Severity:    severity,
			Field:       "status",
			OSValue:     osOrder.Status,
			SAPValue:    sapOrder.Status,
			Description: "Order status mismatch between systems",
			Impact:      "Order fulfillment tracking issues",
			Suggestion:  "Sync status updates or accept expected differences",
		})
	}

	// Compare item count
	if len(osOrder.Items) != len(sapOrder.Items) {
		inconsistencies = append(inconsistencies, DataInconsistency{
			OrderID:     osOrder.ID,
			Type:        "field_mismatch",
			Severity:    "warning",
			Field:       "items_count",
			OSValue:     len(osOrder.Items),
			SAPValue:    len(sapOrder.Items),
			Description: "Different number of items between systems",
			Impact:      "Order fulfillment may be incomplete",
			Suggestion:  "Review item synchronization logic",
		})
	}

	return inconsistencies
}

func (da *DataAnalyzer) isExactMatch(osOrder, sapOrder *models.Order) bool {
	return osOrder.ID == sapOrder.ID &&
		osOrder.CustomerID == sapOrder.CustomerID &&
		math.Abs(osOrder.TotalAmount-sapOrder.TotalAmount) < 0.01 &&
		len(osOrder.Items) == len(sapOrder.Items)
}

func (da *DataAnalyzer) compareOrderFields(osOrder, sapOrder *models.Order) []DataMismatch {
	var mismatches []DataMismatch

	if osOrder.CustomerID != sapOrder.CustomerID {
		mismatches = append(mismatches, DataMismatch{
			OrderID:  osOrder.ID,
			Field:    "customer_id",
			OSValue:  osOrder.CustomerID,
			SAPValue: sapOrder.CustomerID,
		})
	}

	if math.Abs(osOrder.TotalAmount-sapOrder.TotalAmount) > 0.01 {
		mismatches = append(mismatches, DataMismatch{
			OrderID:    osOrder.ID,
			Field:      "total_amount",
			OSValue:    osOrder.TotalAmount,
			SAPValue:   sapOrder.TotalAmount,
			Difference: osOrder.TotalAmount - sapOrder.TotalAmount,
		})
	}

	if osOrder.Status != sapOrder.Status {
		mismatches = append(mismatches, DataMismatch{
			OrderID:  osOrder.ID,
			Field:    "status",
			OSValue:  osOrder.Status,
			SAPValue: sapOrder.Status,
		})
	}

	return mismatches
}

func (da *DataAnalyzer) calculateStatistics(result *ComparisonResult, processingTime time.Duration) ComparisonStatistics {
	stats := ComparisonStatistics{
		AverageProcessingTime: processingTime,
		FieldAccuracy:        make(map[string]float64),
	}

	// Count issues by severity
	for _, inconsistency := range result.Inconsistencies {
		switch inconsistency.Severity {
		case "critical":
			stats.CriticalIssues++
		case "warning":
			stats.WarningIssues++
		case "info":
			stats.InfoIssues++
		}
	}

	// Calculate consistency score
	totalOrders := len(result.OrderServiceData) + len(result.SAPData)
	if totalOrders > 0 {
		totalIssues := len(result.Inconsistencies)
		stats.DataConsistencyScore = math.Max(0, 100-float64(totalIssues*100)/float64(totalOrders))
	}

	// Calculate field accuracy
	fieldCounts := make(map[string]int)
	fieldErrors := make(map[string]int)

	for _, mismatch := range result.Analysis.DataMismatches {
		fieldCounts[mismatch.Field]++
		fieldErrors[mismatch.Field]++
	}

	for field, total := range fieldCounts {
		errors := fieldErrors[field]
		accuracy := float64(total-errors) / float64(total) * 100
		stats.FieldAccuracy[field] = accuracy
	}

	return stats
}

func (da *DataAnalyzer) generateRecommendations(result *ComparisonResult) []string {
	var recommendations []string

	if result.Statistics.CriticalIssues > 0 {
		recommendations = append(recommendations, "🚨 Critical data inconsistencies detected - immediate attention required")
	}

	if len(result.Analysis.MissingInSAP) > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("📤 Migrate %d missing orders to SAP using migration script", len(result.Analysis.MissingInSAP)))
	}

	if len(result.Analysis.MissingInOrderService) > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("📥 Consider importing %d historical SAP orders to Order Service", len(result.Analysis.MissingInOrderService)))
	}

	if result.Analysis.SyncPercentage < 95 {
		recommendations = append(recommendations, "🔄 Run data synchronization to improve consistency")
	}

	if len(result.Analysis.DataMismatches) > 0 {
		recommendations = append(recommendations, "🔍 Review data transformation logic for field mismatches")
	}

	if result.Statistics.DataConsistencyScore < 85 {
		recommendations = append(recommendations, "⚡ Consider implementing real-time data validation")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "✅ Data consistency is excellent - no action required")
	}

	return recommendations
}

func (da *DataAnalyzer) GenerateReport(result *ComparisonResult, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(result, "", "  ")
	case "summary":
		return da.generateSummaryReport(result), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

func (da *DataAnalyzer) generateSummaryReport(result *ComparisonResult) []byte {
	report := fmt.Sprintf(`DATA COMPARISON REPORT
====================
Generated: %s

OVERVIEW
--------
Order Service Orders: %d
SAP Orders: %d
Perfect Matches: %d
Partial Matches: %d
Data Consistency Score: %.2f%%

INCONSISTENCIES
--------------
Critical Issues: %d
Warning Issues: %d
Info Issues: %d

MISSING DATA
-----------
Missing in SAP: %d orders
Missing in Order Service: %d orders

RECOMMENDATIONS
--------------
%s

STATUS: %s
`,
		result.Timestamp.Format(time.RFC3339),
		result.Analysis.TotalOrderService,
		result.Analysis.TotalSAP,
		result.Analysis.PerfectMatches,
		result.Analysis.PartialMatches,
		result.Statistics.DataConsistencyScore,
		result.Statistics.CriticalIssues,
		result.Statistics.WarningIssues,
		result.Statistics.InfoIssues,
		len(result.Analysis.MissingInSAP),
		len(result.Analysis.MissingInOrderService),
		strings.Join(result.Recommendations, "\n"),
		strings.ToUpper(result.Analysis.OverallStatus))

	return []byte(report)
}