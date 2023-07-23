import { t } from 'app/core/internationalization';

export function buildBreakdownString(
  folderCount: number,
  dashboardCount: number,
  libraryPanelCount: number,
  alertRuleCount: number
) {
  console.log({
    folderCount,
    dashboardCount,
    libraryPanelCount,
    alertRuleCount,
  });
  console.log({
    folderCount: folderCount ?? 0,
    dashboardCount: dashboardCount ?? 0,
    libraryPanelCount: libraryPanelCount ?? 0,
    alertRuleCount: alertRuleCount ?? 0,
  });
  const total = folderCount + dashboardCount + libraryPanelCount + alertRuleCount;

  const parts = [];
  if (folderCount) {
    parts.push(t('browse-dashboards.counts.folder', '{{count}} folder', { count: folderCount }));
  }
  if (dashboardCount) {
    parts.push(t('browse-dashboards.counts.dashboard', '{{count}} dashboard', { count: dashboardCount }));
  }
  if (libraryPanelCount) {
    parts.push(t('browse-dashboards.counts.libraryPanel', '{{count}} library panel', { count: libraryPanelCount }));
  }
  if (alertRuleCount) {
    parts.push(t('browse-dashboards.counts.alertRule', '{{count}} alert rule', { count: alertRuleCount }));
  }
  let breakdownString = t('browse-dashboards.counts.total', '{{count}} item', { count: total });
  if (parts.length > 0) {
    breakdownString += `: ${parts.join(', ')}`;
  }
  return breakdownString;
}
