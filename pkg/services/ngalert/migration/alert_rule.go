package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/grafana/grafana/pkg/infra/log"
	legacymodels "github.com/grafana/grafana/pkg/services/alerting/models"
	migmodels "github.com/grafana/grafana/pkg/services/ngalert/migration/models"
	ngmodels "github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/tsdb/graphite"
	"github.com/grafana/grafana/pkg/util"
)

const (
	// ContactLabelTemplate is a private label added to a rule's labels to route it to the correct migrated
	// notification channel.
	ContactLabelTemplate = "__contacts_%s__"
)

func (om *OrgMigration) cleanupDashboardAlerts(ctx context.Context, du *migmodels.DashboardUpgrade) error {
	// Cleanup.
	if du != nil {
		ruleUids := make([]string, 0, len(du.MigratedAlerts))
		for _, pair := range du.MigratedAlerts {
			if pair.AlertRule != nil && pair.AlertRule.UID != "" {
				ruleUids = append(ruleUids, pair.AlertRule.UID)
			}
		}
		if len(ruleUids) > 0 {
			err := om.migrationStore.DeleteAlertRules(ctx, om.orgID, ruleUids...)
			if err != nil {
				return fmt.Errorf("delete existing alert rules: %w", err)
			}
		}

		// Delete newly created folder if one exists and there should be nothing in it.
		if du.NewFolderUID != "" && du.NewFolderUID != du.FolderUID {
			// Remove uid from summary.createdFolders
			found := false
			for i, uid := range om.state.CreatedFolders {
				if uid == du.NewFolderUID {
					om.state.CreatedFolders = append(om.state.CreatedFolders[:i], om.state.CreatedFolders[i+1:]...)
					found = true
					break
				}
			}
			// Safety check to prevent deleting folders that were not created by this migration.
			if found {
				err := om.migrationStore.DeleteFolders(ctx, om.orgID, du.NewFolderUID)
				if err != nil {
					return fmt.Errorf("delete folder '%s': %w", du.NewFolderName, err)
				}
			}
		}
	}
	return nil
}

func addLabelsAndAnnotations(l log.Logger, alert *legacymodels.Alert, dashboardUID string, channels []string) (map[string]string, map[string]string) {
	tags := alert.GetTagsFromSettings()
	lbls := make(map[string]string)

	for _, t := range tags {
		lbls[t.Key] = t.Value
	}

	// Add a label for routing
	lbls[migmodels.UseLegacyChannelsLabel] = "true"
	for _, c := range channels {
		lbls[fmt.Sprintf(ContactLabelTemplate, c)] = "true"
	}

	annotations := make(map[string]string, 4)
	annotations[ngmodels.DashboardUIDAnnotation] = dashboardUID
	annotations[ngmodels.PanelIDAnnotation] = fmt.Sprintf("%v", alert.PanelID)
	annotations["__alertId__"] = fmt.Sprintf("%v", alert.ID)

	message := MigrateTmpl(l.New("field", "message"), alert.Message)
	annotations["message"] = message

	return lbls, annotations
}

// migrateAlert migrates a single dashboard alert from legacy alerting to unified alerting.
func (om *OrgMigration) migrateAlert(ctx context.Context, l log.Logger, alert *legacymodels.Alert, info migmodels.DashboardUpgradeInfo) (*ngmodels.AlertRule, error) {
	l.Debug("Migrating alert rule to Unified Alerting")
	rawSettings, err := json.Marshal(alert.Settings)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}
	var parsedSettings dashAlertSettings
	err = json.Unmarshal(rawSettings, &parsedSettings)
	if err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	cond, err := transConditions(ctx, parsedSettings, alert.OrgID, om.migrationStore)
	if err != nil {
		return nil, fmt.Errorf("transform conditions: %w", err)
	}

	channels := om.extractChannelUIDs(ctx, l, alert.OrgID, parsedSettings)

	lbls, annotations := addLabelsAndAnnotations(l, alert, info.DashboardUID, channels)

	data, err := migrateAlertRuleQueries(l, cond.Data)
	if err != nil {
		return nil, fmt.Errorf("queries: %w", err)
	}

	isPaused := false
	if alert.State == "paused" {
		isPaused = true
	}

	// Here we ensure that the alert rule title is unique within the folder.
	dedupSet := om.AlertTitleDeduplicator(info.NewFolderUID)
	name := truncateRuleName(alert.Name)
	if dedupSet.contains(name) {
		dedupedName := dedupSet.deduplicate(name)
		l.Debug("Duplicate alert rule name detected, renaming", "old_name", name, "new_name", dedupedName)
		name = dedupedName
	}
	dedupSet.add(name)

	dashUID := info.DashboardUID
	ar := &ngmodels.AlertRule{
		OrgID:           alert.OrgID,
		Title:           name,
		UID:             util.GenerateShortUID(),
		Condition:       cond.Condition,
		Data:            data,
		IntervalSeconds: ruleAdjustInterval(alert.Frequency),
		Version:         1,
		NamespaceUID:    info.NewFolderUID,
		DashboardUID:    &dashUID,
		PanelID:         &alert.PanelID,
		RuleGroup:       fmt.Sprintf("%s - %d", info.DashboardName, alert.PanelID), // Unique to this dash alert but still contains useful info.
		For:             alert.For,
		Updated:         time.Now().UTC(),
		Annotations:     annotations,
		Labels:          lbls,
		RuleGroupIndex:  1, // Every rule is in its own group.
		IsPaused:        isPaused,
		NoDataState:     transNoData(l, parsedSettings.NoDataState),
		ExecErrState:    transExecErr(l, parsedSettings.ExecutionErrorState),
	}

	// Label for routing and silences.
	n, v := getLabelForSilenceMatching(ar.UID)
	ar.Labels[n] = v

	if parsedSettings.ExecutionErrorState == string(legacymodels.ExecutionErrorKeepState) {
		if err := om.addErrorSilence(ar); err != nil {
			om.log.Error("Alert migration error: failed to create silence for Error", "rule_name", ar.Title, "err", err)
		}
	}

	if parsedSettings.NoDataState == string(legacymodels.NoDataKeepState) {
		if err := om.addNoDataSilence(ar); err != nil {
			om.log.Error("Alert migration error: failed to create silence for NoData", "rule_name", ar.Title, "err", err)
		}
	}

	return ar, nil
}

// migrateAlertRuleQueries attempts to fix alert rule queries so they can work in unified alerting. Queries of some data sources are not compatible with unified alerting.
func migrateAlertRuleQueries(l log.Logger, data []ngmodels.AlertQuery) ([]ngmodels.AlertQuery, error) {
	result := make([]ngmodels.AlertQuery, 0, len(data))
	for _, d := range data {
		// queries that are expression are not relevant, skip them.
		if d.DatasourceUID == expressionDatasourceUID {
			result = append(result, d)
			continue
		}
		var fixedData map[string]json.RawMessage
		err := json.Unmarshal(d.Model, &fixedData)
		if err != nil {
			return nil, err
		}
		// remove hidden tag from the query (if exists)
		delete(fixedData, "hide")
		fixedData = fixGraphiteReferencedSubQueries(fixedData)
		fixedData = fixPrometheusBothTypeQuery(l, fixedData)
		updatedModel, err := json.Marshal(fixedData)
		if err != nil {
			return nil, err
		}
		d.Model = updatedModel
		result = append(result, d)
	}
	return result, nil
}

// fixGraphiteReferencedSubQueries attempts to fix graphite referenced sub queries, given unified alerting does not support this.
// targetFull of Graphite data source contains the expanded version of field 'target', so let's copy that.
func fixGraphiteReferencedSubQueries(queryData map[string]json.RawMessage) map[string]json.RawMessage {
	fullQuery, ok := queryData[graphite.TargetFullModelField]
	if ok {
		delete(queryData, graphite.TargetFullModelField)
		queryData[graphite.TargetModelField] = fullQuery
	}

	return queryData
}

// fixPrometheusBothTypeQuery converts Prometheus 'Both' type queries to range queries.
func fixPrometheusBothTypeQuery(l log.Logger, queryData map[string]json.RawMessage) map[string]json.RawMessage {
	// There is the possibility to support this functionality by:
	//	- Splitting the query into two: one for instant and one for range.
	//  - Splitting the condition into two: one for each query, separated by OR.
	// However, relying on a 'Both' query instead of multiple conditions to do this in legacy is likely
	// to be unintentional. In addition, this would require more robust operator precedence in classic conditions.
	// Given these reasons, we opt to convert them to range queries and log a warning.

	var instant bool
	if instantRaw, ok := queryData["instant"]; ok {
		if err := json.Unmarshal(instantRaw, &instant); err != nil {
			// Nothing to do here, we can't parse the instant field.
			if isPrometheus, _ := isPrometheusQuery(queryData); isPrometheus {
				l.Info("Failed to parse instant field on Prometheus query", "instant", string(instantRaw), "err", err)
			}
			return queryData
		}
	}
	var rng bool
	if rangeRaw, ok := queryData["range"]; ok {
		if err := json.Unmarshal(rangeRaw, &rng); err != nil {
			// Nothing to do here, we can't parse the range field.
			if isPrometheus, _ := isPrometheusQuery(queryData); isPrometheus {
				l.Info("Failed to parse range field on Prometheus query", "range", string(rangeRaw), "err", err)
			}
			return queryData
		}
	}

	if !instant || !rng {
		// Only apply this fix to 'Both' type queries.
		return queryData
	}

	isPrometheus, err := isPrometheusQuery(queryData)
	if err != nil {
		l.Info("Unable to convert alert rule that resembles a Prometheus 'Both' type query to 'Range'", "err", err)
		return queryData
	}
	if !isPrometheus {
		// Only apply this fix to Prometheus.
		return queryData
	}

	// Convert 'Both' type queries to `Range` queries by disabling the `Instant` portion.
	l.Warn("Prometheus 'Both' type queries are not supported in unified alerting. Converting to range query.")
	queryData["instant"] = []byte("false")

	return queryData
}

// isPrometheusQuery checks if the query is for Prometheus.
func isPrometheusQuery(queryData map[string]json.RawMessage) (bool, error) {
	ds, ok := queryData["datasource"]
	if !ok {
		return false, fmt.Errorf("missing datasource field")
	}
	var datasource struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(ds, &datasource); err != nil {
		return false, fmt.Errorf("parse datasource '%s': %w", string(ds), err)
	}
	if datasource.Type == "" {
		return false, fmt.Errorf("missing type field '%s'", string(ds))
	}
	return datasource.Type == "prometheus", nil
}

func ruleAdjustInterval(freq int64) int64 {
	// 10 corresponds to the SchedulerCfg, but TODO not worrying about fetching for now.
	var baseFreq int64 = 10
	if freq <= baseFreq {
		return 10
	}
	return freq - (freq % baseFreq)
}

func transNoData(l log.Logger, s string) ngmodels.NoDataState {
	switch legacymodels.NoDataOption(s) {
	case legacymodels.NoDataSetOK:
		return ngmodels.OK // values from ngalert/models/rule
	case "", legacymodels.NoDataSetNoData:
		return ngmodels.NoData
	case legacymodels.NoDataSetAlerting:
		return ngmodels.Alerting
	case legacymodels.NoDataKeepState:
		return ngmodels.NoData // "keep last state" translates to no data because we now emit a special alert when the state is "noData". The result is that the evaluation will not return firing and instead we'll raise the special alert.
	default:
		l.Warn("Unable to translate execution of NoData state. Using default execution", "old", s, "new", ngmodels.NoData)
		return ngmodels.NoData
	}
}

func transExecErr(l log.Logger, s string) ngmodels.ExecutionErrorState {
	switch legacymodels.ExecutionErrorOption(s) {
	case "", legacymodels.ExecutionErrorSetAlerting:
		return ngmodels.AlertingErrState
	case legacymodels.ExecutionErrorKeepState:
		// Keep last state is translated to error as we now emit a
		// DatasourceError alert when the state is error
		return ngmodels.ErrorErrState
	case legacymodels.ExecutionErrorSetOk:
		return ngmodels.OkErrState
	default:
		l.Warn("Unable to translate execution of Error state. Using default execution", "old", s, "new", ngmodels.ErrorErrState)
		return ngmodels.ErrorErrState
	}
}

// truncateRuleName truncates the rule name to the maximum allowed length.
func truncateRuleName(daName string) string {
	if len(daName) > store.AlertDefinitionMaxTitleLength {
		return daName[:store.AlertDefinitionMaxTitleLength]
	}
	return daName
}

// extractChannelUIDs extracts the notification channel UIDs from the given legacy dashboard alert parsed settings.
func (om *OrgMigration) extractChannelUIDs(ctx context.Context, l log.Logger, orgID int64, parsedSettings dashAlertSettings) (channelUids []string) {
	// Extracting channel UID/ID.
	for _, ui := range parsedSettings.Notifications {
		// Either id or uid can be defined in the dashboard alert notification settings. See alerting.NewRuleFromDBAlert.
		if ui.ID > 0 {
			uid, err := om.migrationStore.GetAlertNotificationUidWithId(ctx, orgID, ui.ID)
			if err != nil {
				l.Error("Failed to get alert notification UID", "notificationId", ui.ID, "err", err)
			}
			channelUids = append(channelUids, uid)
		} else if ui.UID != "" {
			channelUids = append(channelUids, ui.UID)
		}
	}

	return channelUids
}
