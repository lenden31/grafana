import React, {useEffect, useMemo, useState} from 'react';

import { QueryEditorProps, TimeRange, DateTime } from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import {LinkButton} from '@grafana/ui'

import { PyroscopeDataSource } from '../datasource';
import { PyroscopeDataSourceOptions, Query } from '../types';

const PYROSCOPE_APP_ID = 'grafana-pyroscope-app'

/* Global promise to fetch the pyroscope app settings */
let pyroscopeAppSettings: Promise<PyroscopeAppSettings> | null= null;
/* Global promises to fetch pyroscope datasource settings by uid as encountered */
const pyroscopeDatasourceSettingsByUid: Record<string, Promise<PyroscopeDatasourceSettings>> = {};

export type Props = QueryEditorProps<PyroscopeDataSource, Query, PyroscopeDataSourceOptions>;

/** A subset of the app settings that are relevant for this integration */
type PyroscopeAppSettings = {
  enabled: boolean;
  jsonData: {
    backendUrl: string;
    basicAuthUser: string;
  }
}

/** A subset of the datasource settings that are relevant for this integration */
type PyroscopeDatasourceSettings = {
  url: string;
  basicAuthUser: string | number; // This might be configured as a number, so we will want to ensure compare for string-equivalence
}

export function PyroscopeAppIntegration(props: Props) {

  const [appPlugin, setAppPlugin] = useState<PyroscopeAppSettings>()
  const [datasource, setDatasource] = useState<PyroscopeDatasourceSettings>()

  const {datasource:{uid: datasourceUid}, query, range} = props;

  useEffect(()=>{

    if (pyroscopeAppSettings == null) {
      pyroscopeAppSettings = getBackendSrv().get<PyroscopeAppSettings>(`/api/plugins/${PYROSCOPE_APP_ID}/settings`)
    }

    pyroscopeAppSettings.then(setAppPlugin);
    
  }, [])

  useEffect(()=>{
    let datasourceSettings = pyroscopeDatasourceSettingsByUid[datasourceUid];

    if (datasourceSettings == null) {
      datasourceSettings = getBackendSrv().get<PyroscopeDatasourceSettings>(`/api/datasources/uid/${datasourceUid}`)
      pyroscopeDatasourceSettingsByUid[datasourceUid] = datasourceSettings;
    }

    datasourceSettings.then(setDatasource);

  }, [datasourceUid])

  const queryParam = useMemo(()=>{

    return generateQueryParams(query, range);
  }, [query, range])

  const profilesAppLink = useMemo(()=>isPyroscopeDatasourceCompatibleWithPlugin(datasource, appPlugin) &&
    <LinkButton variant='secondary' icon='external-link-alt' tooltip={'Open query in Profiles App'} target='_blank' href={`/a/${PYROSCOPE_APP_ID}/single?${queryParam}`}>Profiles App</LinkButton>
  , [appPlugin, datasource, queryParam])

  return profilesAppLink || null;
}

export function isPyroscopeDatasourceCompatibleWithPlugin(datasource?: PyroscopeDatasourceSettings, appPlugin?: PyroscopeAppSettings) {
  if (!appPlugin || !datasource) {
    return false;
  }

  return (
    datasource.url === appPlugin.jsonData.backendUrl
    &&
    String(datasource.basicAuthUser) === String(appPlugin.jsonData.basicAuthUser)
  )
}


function stringifyRawTimeRangePart(rawTimeRangePart: DateTime | string) {
  if (typeof rawTimeRangePart === 'string') {
    return rawTimeRangePart;
  }

  // The `unix` result as a string is compatible with Pyroscope's range part format
  return Math.round(rawTimeRangePart.unix()).toString();
}

export function translateGrafanaTimeRangeToPyroscope(timeRange: TimeRange) {
  const from = stringifyRawTimeRangePart(timeRange.raw.from);
  const until = stringifyRawTimeRangePart(timeRange.raw.to);

  return { from, until };
}

export function generateQueryParams(query?: Query, range?: TimeRange) {
  if (!query || !range) {
    return '';
  }
  const {labelSelector, profileTypeId} = query;

  const params = new URLSearchParams();

  if (profileTypeId && profileTypeId !== '') {
    params.set('query', profileTypeId + (labelSelector || ''))
  }
  
  if (range) {
    const {from, until} = translateGrafanaTimeRangeToPyroscope(range);
    params.set('from', from);
    params.set('until', until);
  }

  // TODO figure out what to do with `queryType` (in query)
  // TODO figure out how to represent `groupBy` (in query)
  // Note: We seem to be fine about not having a service_name="..." in the labelSelector-- it just remains unselected in the pyroscope app
  // Note: App plugin likes uses groupBy with service_name, but the query editor doesn't seem to allow service_name as an option. 
  // TODO figure out what to put for `groupByValue`
  // Note: App plugin uses groupByValue=All

  // TODO Code restructure, multiple files
  // TODO Tests; param extraction.

  return params.toString();
}
