package migration

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log/logtest"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
)

func TestMigrateAlertRuleQueries(t *testing.T) {
	tc := []struct {
		name     string
		input    *simplejson.Json
		expected string
		err      error
	}{
		{
			name:     "when a query has a sub query - it is extracted",
			input:    simplejson.NewFromAny(map[string]any{"targetFull": "thisisafullquery", "target": "ahalfquery"}),
			expected: `{"target":"thisisafullquery"}`,
		},
		{
			name:     "when a query does not have a sub query - it no-ops",
			input:    simplejson.NewFromAny(map[string]any{"target": "ahalfquery"}),
			expected: `{"target":"ahalfquery"}`,
		},
		{
			name:     "when query was hidden, it removes the flag",
			input:    simplejson.NewFromAny(map[string]any{"hide": true}),
			expected: `{}`,
		},
		{
			name: "when prometheus both type query, convert to range",
			input: simplejson.NewFromAny(map[string]any{
				"datasource": map[string]string{
					"type": "prometheus",
				},
				"instant": true,
				"range":   true,
			}),
			expected: `{"datasource":{"type":"prometheus"},"instant":false,"range":true}`,
		},
		{
			name: "when prometheus instant type query, do nothing",
			input: simplejson.NewFromAny(map[string]any{
				"datasource": map[string]string{
					"type": "prometheus",
				},
				"instant": true,
			}),
			expected: `{"datasource":{"type":"prometheus"},"instant":true}`,
		},
		{
			name: "when non-prometheus with instant and range, do nothing",
			input: simplejson.NewFromAny(map[string]any{
				"datasource": map[string]string{
					"type": "something",
				},
				"instant": true,
				"range":   true,
			}),
			expected: `{"datasource":{"type":"something"},"instant":true,"range":true}`,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			model, err := tt.input.Encode()
			require.NoError(t, err)
			queries, err := migrateAlertRuleQueries(&logtest.Fake{}, []models.AlertQuery{{Model: model}})
			if tt.err != nil {
				require.Error(t, err)
				require.EqualError(t, err, tt.err.Error())
				return
			}

			require.NoError(t, err)
			r, err := queries[0].Model.MarshalJSON()
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(r))
		})
	}
}

func TestAddMigrationInfo(t *testing.T) {
	tt := []struct {
		name                string
		alert               *dashAlert
		dashboard           string
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name:                "when alert rule tags are a JSON array, they're ignored.",
			alert:               &dashAlert{Id: 43, ParsedSettings: &dashAlertSettings{AlertRuleTags: []string{"one", "two", "three", "four"}}, PanelId: 42},
			dashboard:           "dashboard",
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{"__alertId__": "43", "__dashboardUid__": "dashboard", "__panelId__": "42"},
		},
		{
			name:                "when alert rule tags are a JSON object",
			alert:               &dashAlert{Id: 43, ParsedSettings: &dashAlertSettings{AlertRuleTags: map[string]any{"key": "value", "key2": "value2"}}, PanelId: 42},
			dashboard:           "dashboard",
			expectedLabels:      map[string]string{"key": "value", "key2": "value2"},
			expectedAnnotations: map[string]string{"__alertId__": "43", "__dashboardUid__": "dashboard", "__panelId__": "42"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			labels, annotations := addMigrationInfo(tc.alert, tc.dashboard)
			require.Equal(t, tc.expectedLabels, labels)
			require.Equal(t, tc.expectedAnnotations, annotations)
		})
	}
}

func TestMakeAlertRule(t *testing.T) {
	t.Run("when mapping rule names", func(t *testing.T) {
		t.Run("leaves basic names untouched", func(t *testing.T) {
			m := newTestMigration(t)
			da := createTestDashAlert()
			cnd := createTestDashAlertCondition()

			ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")

			require.NoError(t, err)
			require.Equal(t, da.Name, ar.Title)
			require.Equal(t, ar.Title, ar.RuleGroup)
		})

		t.Run("truncates very long names to max length", func(t *testing.T) {
			m := newTestMigration(t)
			da := createTestDashAlert()
			da.Name = strings.Repeat("a", store.AlertDefinitionMaxTitleLength+1)
			cnd := createTestDashAlertCondition()

			ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")

			require.NoError(t, err)
			require.Len(t, ar.Title, store.AlertDefinitionMaxTitleLength)
			parts := strings.SplitN(ar.Title, "_", 2)
			require.Len(t, parts, 2)
			require.Greater(t, len(parts[1]), 8, "unique identifier should be longer than 9 characters")
			require.Equal(t, store.AlertDefinitionMaxTitleLength-1, len(parts[0])+len(parts[1]), "truncated name + underscore + unique identifier should together be DefaultFieldMaxLength")
			require.Equal(t, ar.Title, ar.RuleGroup)
		})
	})

	t.Run("alert is not paused", func(t *testing.T) {
		m := newTestMigration(t)
		da := createTestDashAlert()
		cnd := createTestDashAlertCondition()

		ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")
		require.NoError(t, err)
		require.False(t, ar.IsPaused)
	})

	t.Run("paused dash alert is paused", func(t *testing.T) {
		m := newTestMigration(t)
		da := createTestDashAlert()
		da.State = "paused"
		cnd := createTestDashAlertCondition()

		ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")
		require.NoError(t, err)
		require.True(t, ar.IsPaused)
	})

	t.Run("use default if execution of NoData is not known", func(t *testing.T) {
		m := newTestMigration(t)
		da := createTestDashAlert()
		da.ParsedSettings.NoDataState = uuid.NewString()
		cnd := createTestDashAlertCondition()

		ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")
		require.Nil(t, err)
		require.Equal(t, models.NoData, ar.NoDataState)
	})

	t.Run("use default if execution of Error is not known", func(t *testing.T) {
		m := newTestMigration(t)
		da := createTestDashAlert()
		da.ParsedSettings.ExecutionErrorState = uuid.NewString()
		cnd := createTestDashAlertCondition()

		ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")
		require.Nil(t, err)
		require.Equal(t, models.ErrorErrState, ar.ExecErrState)
	})

	t.Run("migrate message template", func(t *testing.T) {
		m := newTestMigration(t)
		da := createTestDashAlert()
		da.Message = "Instance ${instance} is down"
		cnd := createTestDashAlertCondition()

		ar, err := m.makeAlertRule(&logtest.Fake{}, cnd, da, "dashboard", "folder")
		require.Nil(t, err)
		expected :=
			"{{- $mergedLabels := mergeLabelValues $values -}}\n" +
				"Instance {{$mergedLabels.instance}} is down"
		require.Equal(t, expected, ar.Annotations["message"])
	})
}

func createTestDashAlert() dashAlert {
	return dashAlert{
		Id:             1,
		Name:           "test",
		ParsedSettings: &dashAlertSettings{},
	}
}

func createTestDashAlertCondition() condition {
	return condition{
		Condition: "A",
	}
}