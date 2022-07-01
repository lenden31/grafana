import { css } from '@emotion/css';
import React, { useState } from 'react';

import { DataFrame, DataFrameView, GrafanaTheme2 } from '@grafana/data';
import { config } from '@grafana/runtime';
import { Button, Card, FilterInput, Icon, IconName, TagList, useStyles2, VerticalGroup } from '@grafana/ui';

import { StorageView } from './types';

interface Props {
  root: DataFrame;
  onPathChange: (p: string) => void;
  setView: (v: StorageView) => void;
}

interface RootFolder {
  name: string;
  title: string;
  storageType: string;
  description: string;
  readOnly: boolean;
  builtIn: boolean;
}

export function RootView({ root, onPathChange, setView }: Props) {
  const styles = useStyles2(getStyles);
  const [searchQuery, setSearchQuery] = useState<string>('');
  const view = new DataFrameView<RootFolder>(root);
  let base = location.pathname;
  if (!base.endsWith('/')) {
    base += '/';
  }

  return (
    <div>
      <div className="page-action-bar">
        <div className="gf-form gf-form--grow">
          <FilterInput placeholder="Search Storage" value={searchQuery} onChange={setSearchQuery} />
        </div>
        <Button className="pull-right" onClick={() => setView(StorageView.AddRoot)}>
          Add Root
        </Button>
        {config.featureToggles.export && (
          <Button className="pull-right" onClick={() => setView(StorageView.Export)}>
            Export
          </Button>
        )}
      </div>
      <VerticalGroup>
        {view.map((v) => (
          <Card key={v.name} href={`admin/storage/${v.name}/`}>
            <Card.Heading>{v.title ?? v.name}</Card.Heading>
            <Card.Meta className={styles.clickable}>{v.description}</Card.Meta>
            <Card.Tags className={styles.clickable}>
              <TagList tags={getTags(v)} />
            </Card.Tags>
            <Card.Figure className={styles.clickable}>
              <Icon name={getIconName(v.storageType)} size="xxxl" className={styles.secondaryTextColor} />
            </Card.Figure>
          </Card>
        ))}
      </VerticalGroup>
    </div>
  );
}

function getStyles(theme: GrafanaTheme2) {
  return {
    secondaryTextColor: css`
      color: ${theme.colors.text.secondary};
    `,
    clickable: css`
      pointer-events: none;
    `,
  };
}

function getTags(v: RootFolder) {
  const tags: string[] = [];
  if (v.builtIn) {
    tags.push('Builtin');
  }
  if (v.readOnly) {
    tags.push('Read only');
  }
  return tags;
}

export function getIconName(type: string): IconName {
  switch (type) {
    case 'git':
      return 'code-branch';
    case 'disk':
      return 'folder-open';
    case 'sql':
      return 'database';
    default:
      return 'folder-open';
  }
}
