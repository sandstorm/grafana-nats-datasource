import React, {PureComponent} from 'react';
import {InlineField, Input, Select} from '@grafana/ui';
import {
    QueryEditorProps, SelectableValue
} from '@grafana/data';
import {DataSource} from '../datasource';
import {MyDataSourceOptions, MyQuery, QueryTypeOptions} from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

function onChange(props: Props, fieldName: string) {
    return (event: React.SyntheticEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
        props.onChange({...props.query, [fieldName]: event.currentTarget.value});
        props.onRunQuery();
    }
}

function onSelectChange(props: Props, fieldName: string) {
    return (selected: SelectableValue) => {
        props.onChange({...props.query, [fieldName]: selected.value});
        props.onRunQuery();
    }
}

export class QueryEditor extends PureComponent<Props> {
    render() {
        const query = this.props.query;

        return (
            <div className="gf-form-group">
                <div className="gf-form">
                    <InlineField label="Query Type" tooltip="In which way do we interact with NATS">
                        <Select
                            options={QueryTypeOptions}
                            value={query.queryType}
                            onChange={onSelectChange(this.props, 'queryType')}
                        />
                    </InlineField>
                </div>
                <div className="gf-form">
                    <InlineField label="NATS Subject" tooltip="the subject to request - f.e. foo.bar.baz">
                        <Input
                            className="width-27"
                            value={query.natsSubject}
                            onChange={onChange(this.props, 'natsSubject')}
                        />
                    </InlineField>
                </div>
                <div className="gf-form">
                    <InlineField label="Request Timeout">
                        <Input
                            className="width-2"
                            value={query.requestTimeout}
                            onChange={onChange(this.props, 'requestTimeout')}
                        />
                    </InlineField>
                </div>
            </div>
        );
    }
}
