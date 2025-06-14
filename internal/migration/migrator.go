package migration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jogardn/strangler-demo/internal/orders"
	"github.com/jogardn/strangler-demo/internal/sap"
	"github.com/jogardn/strangler-demo/pkg/models"
	"github.com/sirupsen/logrus"
)

type DataMigrator struct {
	orderServiceClient *orders.OrderServiceClient
	sapClient          *sap.Client
	logger             *logrus.Logger
	config             MigrationConfig
}

type MigrationConfig struct {
	BatchSize      int           `json:"batch_size"`
	Concurrency    int           `json:"concurrency"`
	DelayBetween   time.Duration `json:"delay_between"`
	DryRun         bool          `json:"dry_run"`
	SkipExisting   bool          `json:"skip_existing"`
	IncludeItems   bool          `json:"include_items"`
	Direction      string        `json:"direction"` // "to_sap", "to_order_service", "bidirectional"
}

type MigrationResult struct {
	TotalOrders      int                    `json:"total_orders"`
	SuccessfulMigrations int               `json:"successful_migrations"`
	FailedMigrations int                   `json:"failed_migrations"`
	SkippedOrders    int                   `json:"skipped_orders"`
	ProcessingTime   time.Duration         `json:"processing_time"`
	ErrorDetails     []MigrationError      `json:"error_details"`
	Statistics       MigrationStatistics   `json:"statistics"`
	DryRun          bool                   `json:"dry_run"`
	Timestamp       time.Time              `json:"timestamp"`
}

type MigrationError struct {
	OrderID   string `json:"order_id"`
	Error     string `json:"error"`
	Severity  string `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

type MigrationStatistics struct {
	OrdersPerSecond     float64 `json:"orders_per_second"`
	AverageOrderSize    float64 `json:"average_order_size"`
	LargestOrder        float64 `json:"largest_order"`
	DataVolumeProcessed int64   `json:"data_volume_processed"`
}

func NewDataMigrator(orderServiceClient *orders.OrderServiceClient, sapClient *sap.Client, logger *logrus.Logger) *DataMigrator {
	return &DataMigrator{
		orderServiceClient: orderServiceClient,
		sapClient:          sapClient,
		logger:             logger,
		config: MigrationConfig{
			BatchSize:      50,
			Concurrency:    5,
			DelayBetween:   100 * time.Millisecond,
			DryRun:         false,
			SkipExisting:   true,
			IncludeItems:   true,
			Direction:      "bidirectional",
		},
	}
}

func (dm *DataMigrator) SetConfig(config MigrationConfig) {
	dm.config = config
	dm.logger.WithFields(logrus.Fields{
		"batch_size":    config.BatchSize,
		"concurrency":   config.Concurrency,
		"direction":     config.Direction,
		"dry_run":       config.DryRun,
	}).Info("Migration configuration updated")
}

func (dm *DataMigrator) MigrateHistoricalOrders(ctx context.Context) (*MigrationResult, error) {
	startTime := time.Now()
	
	dm.logger.Info("Starting historical order migration")

	result := &MigrationResult{
		ErrorDetails: []MigrationError{},
		DryRun:      dm.config.DryRun,
		Timestamp:   time.Now(),
	}

	// Get orders from both systems
	osOrders, err := dm.orderServiceClient.GetOrders()
	if err != nil {
		return nil, fmt.Errorf("failed to get orders from Order Service: %w", err)
	}

	sapOrders, err := dm.sapClient.GetOrders()
	if err != nil {
		return nil, fmt.Errorf("failed to get orders from SAP: %w", err)
	}

	dm.logger.WithFields(logrus.Fields{
		"order_service_count": len(osOrders),
		"sap_count":          len(sapOrders),
	}).Info("Retrieved orders from both systems")

	// Create lookup maps
	osMap := make(map[string]*models.Order)
	sapMap := make(map[string]*models.Order)
	
	for i := range osOrders {
		osMap[osOrders[i].ID] = &osOrders[i]
	}
	
	for i := range sapOrders {
		sapMap[sapOrders[i].ID] = &sapOrders[i]
	}

	// Determine what needs to be migrated
	var toMigrateToSAP []models.Order
	var toMigrateToOS []models.Order

	if dm.config.Direction == "to_sap" || dm.config.Direction == "bidirectional" {
		toMigrateToSAP = dm.findOrdersToMigrate(osMap, sapMap)
	}

	if dm.config.Direction == "to_order_service" || dm.config.Direction == "bidirectional" {
		toMigrateToOS = dm.findOrdersToMigrate(sapMap, osMap)
	}

	result.TotalOrders = len(toMigrateToSAP) + len(toMigrateToOS)

	if result.TotalOrders == 0 {
		dm.logger.Info("No orders need migration")
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	dm.logger.WithFields(logrus.Fields{
		"to_sap":            len(toMigrateToSAP),
		"to_order_service":  len(toMigrateToOS),
		"total":            result.TotalOrders,
	}).Info("Orders identified for migration")

	// Perform migrations
	if len(toMigrateToSAP) > 0 {
		sapResult := dm.migrateOrdersToSAP(ctx, toMigrateToSAP)
		dm.mergeResults(result, sapResult)
	}

	if len(toMigrateToOS) > 0 {
		osResult := dm.migrateOrdersToOrderService(ctx, toMigrateToOS)
		dm.mergeResults(result, osResult)
	}

	result.ProcessingTime = time.Since(startTime)
	result.Statistics = dm.calculateStatistics(result, osOrders, sapOrders)

	dm.logger.WithFields(logrus.Fields{
		"successful": result.SuccessfulMigrations,
		"failed":     result.FailedMigrations,
		"skipped":    result.SkippedOrders,
		"duration":   result.ProcessingTime,
	}).Info("Migration completed")

	return result, nil
}

func (dm *DataMigrator) findOrdersToMigrate(sourceMap, targetMap map[string]*models.Order) []models.Order {
	var ordersToMigrate []models.Order
	
	for orderID, order := range sourceMap {
		if _, exists := targetMap[orderID]; !exists || !dm.config.SkipExisting {
			ordersToMigrate = append(ordersToMigrate, *order)
		}
	}
	
	return ordersToMigrate
}

func (dm *DataMigrator) migrateOrdersToSAP(ctx context.Context, orders []models.Order) *MigrationResult {
	result := &MigrationResult{
		ErrorDetails: []MigrationError{},
		DryRun:      dm.config.DryRun,
	}

	dm.logger.WithField("count", len(orders)).Info("Migrating orders to SAP")

	if dm.config.DryRun {
		dm.logger.Info("DRY RUN: Would migrate orders to SAP")
		result.SuccessfulMigrations = len(orders)
		return result
	}

	// Process in batches with concurrency
	batches := dm.createBatches(orders)
	
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, dm.config.Concurrency)
	resultChan := make(chan *MigrationResult, len(batches))

	for _, batch := range batches {
		wg.Add(1)
		go func(orderBatch []models.Order) {
			defer wg.Done()
			semaphore <- struct{}{}
			
			batchResult := dm.processBatchToSAP(ctx, orderBatch)
			resultChan <- batchResult
			
			<-semaphore
			time.Sleep(dm.config.DelayBetween)
		}(batch)
	}

	wg.Wait()
	close(resultChan)

	// Collect results
	for batchResult := range resultChan {
		dm.mergeResults(result, batchResult)
	}

	return result
}

func (dm *DataMigrator) migrateOrdersToOrderService(ctx context.Context, orders []models.Order) *MigrationResult {
	result := &MigrationResult{
		ErrorDetails: []MigrationError{},
		DryRun:      dm.config.DryRun,
	}

	dm.logger.WithField("count", len(orders)).Info("Migrating orders to Order Service")

	if dm.config.DryRun {
		dm.logger.Info("DRY RUN: Would migrate orders to Order Service")
		result.SuccessfulMigrations = len(orders)
		return result
	}

	// Process in batches with concurrency
	batches := dm.createBatches(orders)
	
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, dm.config.Concurrency)
	resultChan := make(chan *MigrationResult, len(batches))

	for _, batch := range batches {
		wg.Add(1)
		go func(orderBatch []models.Order) {
			defer wg.Done()
			semaphore <- struct{}{}
			
			batchResult := dm.processBatchToOrderService(ctx, orderBatch)
			resultChan <- batchResult
			
			<-semaphore
			time.Sleep(dm.config.DelayBetween)
		}(batch)
	}

	wg.Wait()
	close(resultChan)

	// Collect results
	for batchResult := range resultChan {
		dm.mergeResults(result, batchResult)
	}

	return result
}

func (dm *DataMigrator) createBatches(orders []models.Order) [][]models.Order {
	var batches [][]models.Order
	
	for i := 0; i < len(orders); i += dm.config.BatchSize {
		end := i + dm.config.BatchSize
		if end > len(orders) {
			end = len(orders)
		}
		batches = append(batches, orders[i:end])
	}
	
	return batches
}

func (dm *DataMigrator) processBatchToSAP(ctx context.Context, orders []models.Order) *MigrationResult {
	result := &MigrationResult{
		ErrorDetails: []MigrationError{},
	}

	for _, order := range orders {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		_, err := dm.sapClient.CreateOrder(&order)
		if err != nil {
			result.FailedMigrations++
			result.ErrorDetails = append(result.ErrorDetails, MigrationError{
				OrderID:   order.ID,
				Error:     err.Error(),
				Severity:  "error",
				Timestamp: time.Now(),
			})
			dm.logger.WithError(err).WithField("order_id", order.ID).Error("Failed to migrate order to SAP")
		} else {
			result.SuccessfulMigrations++
			dm.logger.WithField("order_id", order.ID).Debug("Successfully migrated order to SAP")
		}
	}

	return result
}

func (dm *DataMigrator) processBatchToOrderService(ctx context.Context, orders []models.Order) *MigrationResult {
	result := &MigrationResult{
		ErrorDetails: []MigrationError{},
	}

	for _, order := range orders {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		// Create historical order without publishing events
		_, err := dm.orderServiceClient.CreateOrderHistorical(&order)
		if err != nil {
			result.FailedMigrations++
			result.ErrorDetails = append(result.ErrorDetails, MigrationError{
				OrderID:   order.ID,
				Error:     err.Error(),
				Severity:  "error",
				Timestamp: time.Now(),
			})
			dm.logger.WithError(err).WithField("order_id", order.ID).Error("Failed to migrate order to Order Service")
		} else {
			result.SuccessfulMigrations++
			dm.logger.WithField("order_id", order.ID).Debug("Successfully migrated order to Order Service")
		}
	}

	return result
}

func (dm *DataMigrator) mergeResults(target, source *MigrationResult) {
	target.SuccessfulMigrations += source.SuccessfulMigrations
	target.FailedMigrations += source.FailedMigrations
	target.SkippedOrders += source.SkippedOrders
	target.ErrorDetails = append(target.ErrorDetails, source.ErrorDetails...)
}

func (dm *DataMigrator) calculateStatistics(result *MigrationResult, osOrders, sapOrders []models.Order) MigrationStatistics {
	stats := MigrationStatistics{}

	if result.ProcessingTime > 0 {
		stats.OrdersPerSecond = float64(result.SuccessfulMigrations) / result.ProcessingTime.Seconds()
	}

	// Calculate average order size
	var totalAmount float64
	orderCount := len(osOrders) + len(sapOrders)
	
	for _, order := range osOrders {
		totalAmount += order.TotalAmount
		if order.TotalAmount > stats.LargestOrder {
			stats.LargestOrder = order.TotalAmount
		}
	}
	
	for _, order := range sapOrders {
		totalAmount += order.TotalAmount
		if order.TotalAmount > stats.LargestOrder {
			stats.LargestOrder = order.TotalAmount
		}
	}

	if orderCount > 0 {
		stats.AverageOrderSize = totalAmount / float64(orderCount)
	}

	// Estimate data volume (rough calculation)
	stats.DataVolumeProcessed = int64(result.SuccessfulMigrations * 1024) // Assume ~1KB per order

	return stats
}

func (dm *DataMigrator) ValidateMigration(ctx context.Context) (*ValidationResult, error) {
	dm.logger.Info("Starting post-migration validation")

	// Get fresh data from both systems
	osOrders, err := dm.orderServiceClient.GetOrders()
	if err != nil {
		return nil, fmt.Errorf("failed to get orders from Order Service: %w", err)
	}

	sapOrders, err := dm.sapClient.GetOrders()
	if err != nil {
		return nil, fmt.Errorf("failed to get orders from SAP: %w", err)
	}

	validation := &ValidationResult{
		TotalOrderService: len(osOrders),
		TotalSAP:         len(sapOrders),
		ValidationTime:   time.Now(),
	}

	// Check for missing orders
	osMap := make(map[string]bool)
	for _, order := range osOrders {
		osMap[order.ID] = true
	}

	sapMap := make(map[string]bool)
	for _, order := range sapOrders {
		sapMap[order.ID] = true
	}

	for orderID := range osMap {
		if !sapMap[orderID] {
			validation.MissingInSAP = append(validation.MissingInSAP, orderID)
		}
	}

	for orderID := range sapMap {
		if !osMap[orderID] {
			validation.MissingInOrderService = append(validation.MissingInOrderService, orderID)
		}
	}

	validation.SyncPercentage = dm.calculateSyncPercentage(len(osOrders), len(sapOrders), len(validation.MissingInSAP), len(validation.MissingInOrderService))
	validation.IsValid = validation.SyncPercentage >= 95.0

	dm.logger.WithFields(logrus.Fields{
		"sync_percentage":     validation.SyncPercentage,
		"missing_in_sap":      len(validation.MissingInSAP),
		"missing_in_os":       len(validation.MissingInOrderService),
		"validation_passed":   validation.IsValid,
	}).Info("Migration validation completed")

	return validation, nil
}

func (dm *DataMigrator) calculateSyncPercentage(osCount, sapCount, missingInSAP, missingInOS int) float64 {
	totalUnique := osCount + sapCount - missingInSAP - missingInOS
	if totalUnique == 0 {
		return 100.0
	}
	synchronized := totalUnique - missingInSAP - missingInOS
	return float64(synchronized) / float64(totalUnique) * 100.0
}

type ValidationResult struct {
	TotalOrderService      int       `json:"total_order_service"`
	TotalSAP              int       `json:"total_sap"`
	MissingInSAP          []string  `json:"missing_in_sap"`
	MissingInOrderService []string  `json:"missing_in_order_service"`
	SyncPercentage        float64   `json:"sync_percentage"`
	IsValid               bool      `json:"is_valid"`
	ValidationTime        time.Time `json:"validation_time"`
}