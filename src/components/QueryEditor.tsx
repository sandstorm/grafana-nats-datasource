import React, { ChangeEvent, PureComponent } from 'react';
import { LegacyForms } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

const { FormField } = LegacyForms;

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

function onChange(props: Props, fieldName: string) {
    return (event: React.SyntheticEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
        props.onChange({... props.query, [fieldName]: event.currentTarget.value});
        props.onRunQuery();
    }
}

export class QueryEditor extends PureComponent<Props> {
  render() {
    const { queryText, constant } = this.props.query;

    return (
      <div className="gf-form">
        <FormField
          width={4}
          value={constant}
          onChange={this.onConstantChange}
          label="Constant"
          type="number"
          step="0.1"
        />
        <FormField
          labelWidth={8}
          value={queryText || ''}
          onChange={this.onQueryTextChange}
          label="Query Text"
          tooltip="Not used yet"
        />
      </div>
    );
  }
}
