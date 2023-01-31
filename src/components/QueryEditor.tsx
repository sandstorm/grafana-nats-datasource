import React, {PureComponent} from 'react';
import {Field, InlineField, Input, RadioButtonGroup} from '@grafana/ui';
import {
    QueryEditorProps
} from '@grafana/data';
import {DataSource} from '../datasource';
import {MyDataSourceOptions, MyQuery, QueryTypeOptions, QueryTypes} from '../types';
import {TamarinCodeEditorField} from "./TamarinCodeEditorField";

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

function onChange(props: Props, fieldName: string) {
    return (event: React.SyntheticEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
        props.onChange({...props.query, [fieldName]: event.currentTarget.value});
        props.onRunQuery();
    }
}

function onChangeTamarin(props: Props, fieldName: string) {
    return (value: string) => {
        props.onChange({...props.query, [fieldName]: value});
        props.onRunQuery();
    }
}



function onQueryTypeChange<TVal>(props: Props, fieldName: string) {
    return (selected: TVal) => {
        props.onChange({...props.query, [fieldName]: selected});
        props.onRunQuery();
    }
}

export class QueryEditor extends PureComponent<Props> {
    render() {
        const query = this.props.query;

        return (
            <div>
                <Field label="Query Type" description="How do we interact with the NAtS system">
                    <RadioButtonGroup<QueryTypes>
                        options={QueryTypeOptions}
                        value={query.queryType}
                        onChange={onQueryTypeChange(this.props, 'queryType')}
                    />
                </Field>
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
                <div className="gf-form">
                    <Field label="Optional Tamarin Processing Function">
                        <TamarinCodeEditorField
                            expression={query.tamarinFn}
                            onChange={onChangeTamarin(this.props, 'tamarinFn')}

                        />
                    </Field>
                </div>
            </div>
        );
    }
}
