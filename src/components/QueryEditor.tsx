import React, {PureComponent} from 'react';
import {Alert, ButtonCascader, CascaderOption, Field, Input, RadioButtonGroup} from '@grafana/ui';
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

type SCRIPT_IDS = "default" | "headers";

const scripts: {  [prop in SCRIPT_IDS]: string} = {
    default: `
        // This script is by default used in the backend if no script is given.
    
        // msg.Data contains the received NATS message as string.
        // json.unmarshal converts the message to a Map.
        // by default, the last line of a script is returned automatically.
        json.unmarshal(msg.Data)
    `,
    headers: `
        // You can covert NATS message headers to columns (and in the same way, do any kind of calculation

        // Workaround: unwrap() is needed to convert the Result object to a plain map.
        row := json.unmarshal(msg.Data).unwrap()
        row["otherHeader"] = msg.Header.Get("My-Header")    
        
        return row
    `,
};


function explanationForQueryType(queryType: QueryTypes): { title: string, content: React.ReactNode, tamarinLabel: string, tamarinDescription: React.ReactNode, tamarinExamples?: CascaderOption[] } {
    if (queryType == "REQUEST_REPLY") {
        return {
            title: 'Request/Reply mode explained',
            content: <>
                <p><a href="https://docs.nats.io/nats-concepts/core-nats/reqreply" target="_blank">NATS
                    Request/Reply</a>:
                    Sends a request on the given subject with an empty payload, and <em>renders the single
                        response</em> (delivered to the _INBOX).</p>

                <p>JSON messages can be rendered directly - nested JSON is flattened. Example messages: <br/>
                    <code>{'{"key1": "val1", "key2": "value2"}'}</code><br/>
                    <code>{'[{"key1": "val1", "key2": "value2"}, {"key1": "val3"}]'}</code></p>

                <p>You can post-process each message via the Tamarin script language.</p>
            </>,
            tamarinLabel: 'Response Mapping Script',
            tamarinDescription: <>
                Input: <code>msg</code> contains the received message as a <a
                href="https://pkg.go.dev/github.com/nats-io/nats.go#Msg" target="_blank">nats.Msg</a>.<br/>
                Supported Return values: A map <code>{'{k: "v"}'}</code>, a list of maps <code>{'[{k: "v"}]'}</code>,
                a <a href="https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go@v0.147.0/data#Frame"
                     target="_blank">data.Frame</a>.<br/>
                <a href="https://cloudcmds.github.io/tamarin/" target="_blank">Guide to Tamarin:</a> Tamarin is a
                scripting language, a hybrid between Golang and JS.
            </>,
            tamarinExamples: [
                {
                    label: 'Default script',
                    title: 'The most simple script which is used by default on the backend.',
                    value: "default",
                },
                {
                    label: 'display NATS message headers',
                    title: 'display NATS message headers in Grafana',
                    value: "headers"
                }
            ]
        };
    }
    if (queryType == "SUBSCRIBE") {
        return {
            title: 'Subscribe mode explained',
            content: <>
                <p><a href="https://docs.nats.io/nats-concepts/core-nats/pubsub" target="_blank">NATS
                    Publish/Subscribe</a>:
                    Listen to messages on the given subject pattern, and sends them via
                    <a href="https://grafana.com/docs/grafana/latest/setup-grafana/set-up-grafana-live/"
                       target="_blank">Grafana Live</a>
                    to the frontend.</p>

                <p>JSON messages can be rendered directly - nested JSON is flattened. Example messages: <br/>
                    <code>{'{"key1": "val1", "key2": "value2"}'}</code></p>

                <p>You can post-process each message via the Tamarin script language.</p>
            </>,
            tamarinLabel: 'Message Mapping Script',
            tamarinDescription: <>
                Input: <code>msg</code> contains the received message as a <a
                href="https://pkg.go.dev/github.com/nats-io/nats.go#Msg" target="_blank">nats.Msg</a>.<br/>
                Supported Return values: A map <code>{'{k: "v"}'}</code>.<br/>
                <a href="https://cloudcmds.github.io/tamarin/" target="_blank">Guide to Tamarin:</a> Tamarin is a
                scripting language, a hybrid between Golang and JS.
            </>,
            tamarinExamples: [
                {
                    label: 'Default script',
                    title: 'The most simple script which is used by default on the backend.',
                    value: "default"
                },
                {
                    label: 'display NATS message headers',
                    title: 'display NATS message headers in Grafana',
                    value: "headers"
                }
            ]

        };
    }
    if (queryType == "SCRIPT") {
        return {
            title: 'Script mode explained',
            content: <>
                <p>For advanced use cases, a free-form script can be used, which directly controls how messages
                    are sent and how their responses are processed:</p>

                - to <em>handle multiple replies to the same request</em>,<br/>
                - to send multiple <em>dependent requests</em>,<br/>
                - to <em>collect/reduce multiple responses</em> into a single UI response,<br/>
                - other advanced cases.<br/>

                <p>The free-form script can return results directly or <em>stream them</em> to the UI. See the inline
                    script examples, they are heavily commented.</p>
            </>,
            tamarinLabel: 'Response Mapping Script',
            tamarinDescription:
                <>
                    Input: <code>nc</code> the <a
                    href="https://pkg.go.dev/github.com/nats-io/nats.go#Conn" target="_blank">nats.Conn</a> you can use
                    to
                    <a href="https://pkg.go.dev/github.com/nats-io/nats.go#Conn.Subscribe">nc.Subscribe()</a>,
                    <a href="https://pkg.go.dev/github.com/nats-io/nats.go#Conn.Request">nc.Request()</a><br/> (or any
                    other interaction).<br/>
                    Supported Return values: <a href="https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go@v0.147.0/data#Frame"
                    target="_blank">data.Frame</a> or an error.<br/>
                    <a href="https://cloudcmds.github.io/tamarin/" target="_blank">Guide to Tamarin:</a> Tamarin is
                    a scripting language, a hybrid between Golang and JS.
                </>,
            tamarinExamples: [

            ]

        };
    }
    assertUnreachable(queryType);
}

function assertUnreachable(x: never): never {
    throw new Error("Didn't expect to get here");
}

export class QueryEditor extends PureComponent<Props> {
    render() {
        const query = this.props.query;

        const explanation = explanationForQueryType(query.queryType);
        return (
            <div>
                <div className="gf-form">
                    <Field label="Query Type" description="How do we interact with the NAtS system">
                        <RadioButtonGroup<QueryTypes>
                            options={QueryTypeOptions}
                            value={query.queryType}
                            onChange={onQueryTypeChange(this.props, 'queryType')}
                        />
                    </Field>
                </div>
                <div className="gf-form">
                    <Alert title={explanation.title} severity="info">
                        {explanation.content}
                    </Alert>
                </div>
                <div className="gf-form">
                    <Field label="NATS Subject" description="the subject to request - f.e. foo.bar.baz">
                        <Input
                            className="width-27"
                            value={query.natsSubject}
                            onChange={onChange(this.props, 'natsSubject')}
                        />
                    </Field>
                </div>
                <div className="gf-form">
                    <Field label="Request Timeout">
                        <Input
                            className="width-2"
                            value={query.requestTimeout}
                            onChange={onChange(this.props, 'requestTimeout')}
                        />
                    </Field>
                </div>
                <div className="gf-form">
                    <Field label={explanation.tamarinLabel} style={{width: '100%'}}
                           description={explanation.tamarinDescription}>
                        <TamarinCodeEditorField
                            expression={query.jsFn}
                            onChange={onChangeTamarin(this.props, 'tamarinFn')}
                        />
                    </Field>
                    {explanation.tamarinExamples ?
                        <ButtonCascader options={explanation.tamarinExamples} onChange={(value) => onChangeTamarin(this.props, 'tamarinFn')(scripts[value[0] as SCRIPT_IDS])}>
                            Example Code
                        </ButtonCascader> : null}
                </div>
            </div>
        );
    }
}
