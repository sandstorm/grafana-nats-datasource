import React, {PureComponent} from 'react';
import {Alert, ButtonCascader, CascaderOption, Field, FieldSet, Input, RadioButtonGroup} from '@grafana/ui';
import {
    QueryEditorProps
} from '@grafana/data';
import {DataSource} from '../datasource';
import {MyDataSourceOptions, MyQuery, QueryTypeOptions, QueryTypes} from '../types';
import {JavaScriptCodeEditorField} from "./JavaScriptCodeEditorField";

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

function onChange(props: Props, fieldName: string) {
    return (event: React.SyntheticEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
        props.onChange({...props.query, [fieldName]: event.currentTarget.value});
        props.onRunQuery();
    }
}

function onChangeJs(props: Props, fieldName: string) {
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

type SCRIPT_IDS = "default" | "headers" | "scripting_multipleRequests" | "scripting_multipleResponses";

const scripts: {  [prop in SCRIPT_IDS]: string} = {
    default: `
        // This script is by default used in the backend if no script is given.
    
        // msg.Data contains the received NATS message as string.
        // by default, the last line of a script is returned automatically.
        JSON.parse(msg.Data)
    `,
    headers: `
        // You can covert NATS message headers to columns (and in the same way, do any kind of calculation

        row = JSON.parse(msg.Data)
        row["otherHeader"] = msg.Header.Get("My-Header")    
        
        return row
    `,
    scripting_multipleRequests: `
        // do two requests on different NATS subjects (json1 and json2)
        const msg1 = nc.Request("json1", "", "50ms");
        const msg2 = nc.Request("json2", "", "50ms");
        
        // parse the response data as JSON
        const parsed1 = JSON.parse(msg1.Data);
        const parsed2 = JSON.parse(msg2.Data);
        
        // return the concatenated list
        return [parsed1, parsed2];
    `,
    scripting_multipleResponses: `
        // Sometimes, you receive *multiple responses* for a single request, f.e. when
        // triggering $SYS.REQ.SERVER.PING in the SYS account, you will receive one answer
        // per server.
        //
        // That's why we manually create an inbox for the reply; and poll it as
        // long as there are messages.
        const result = [];
        
        const inbox = nc.NewInbox();
        // The ordering is crucial: we first need to create the subscription, before
        // sending the request (otherwise we might miss the response).
        const subscription = nc.SubscribeSync(inbox);
        nc.PublishRequest("$SYS.REQ.SERVER.PING", inbox, "");
        while(true) {
          // we poll until we do not receive a message anymore within the given timeout.
          const msg = subscription.NextMsg("50ms");
          if (!msg) {
            // ... when this happens, we return the accumulated result.
            return result;
          }
          // here, we parse the given message.
          const parsed = JSON.parse(msg.Data);
          delete parsed.statsz.routes;
          result.push(parsed);
    }
    `
};


function explanationForQueryType(queryType: QueryTypes): { title: string, content: React.ReactNode, natsSubjectDescription?: string, mapFnLabel: string, mapFnDescription: React.ReactNode, mapFnExamples?: Array<CascaderOption&{value: SCRIPT_IDS}> } {
    if (queryType === "REQUEST_REPLY") {
        return {
            title: 'Request/Reply mode explained',
            content: <>
                <p><a href="https://docs.nats.io/nats-concepts/core-nats/reqreply" target="_blank" rel="noreferrer">NATS
                    Request/Reply</a>:
                    Sends a request on the given subject with an empty payload, and <em>renders the single
                        response</em> (delivered to the _INBOX).</p>

                <p>JSON messages can be rendered directly - nested JSON is flattened. Example messages: <br/>
                    <code>{'{"key1": "val1", "key2": "value2"}'}</code><br/>
                    <code>{'[{"key1": "val1", "key2": "value2"}, {"key1": "val3"}]'}</code></p>

                <p>You can post-process each message via JavaScript.</p>
            </>,
            natsSubjectDescription: 'the subject to request - f.e. foo.bar.baz',
            mapFnLabel: 'Response Mapping JavaScript',
            mapFnDescription: <>
                Input: <code>msg</code> contains the received message as a <a
                href="https://pkg.go.dev/github.com/nats-io/nats.go#Msg" target="_blank" rel="noreferrer">nats.Msg</a>.<br/>
                Supported Return values: A map <code>{'{k: "v"}'}</code>, a list of maps <code>{'[{k: "v"}]'}</code>,
                a <a href="https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go@v0.147.0/data#Frame"
                     target="_blank" rel="noreferrer">data.Frame</a>.
            </>,
            mapFnExamples: [
                {
                    label: 'Default script',
                    title: 'The most simple script which is used by default on the backend.',
                    value: "default" as "default"
                },
                {
                    label: 'display NATS message headers',
                    title: 'display NATS message headers in Grafana',
                    value: "headers" as "headers"
                }
            ]
        };
    }
    if (queryType === "SUBSCRIBE") {
        return {
            title: 'Subscribe mode explained',
            content: <>
                <p><a href="https://docs.nats.io/nats-concepts/core-nats/pubsub" target="_blank" rel="noreferrer">NATS
                    Subscribe</a>:
                    Listen to messages on the given subject pattern, and sends them via
                    <a href="https://grafana.com/docs/grafana/latest/setup-grafana/set-up-grafana-live/"
                       target="_blank" rel="noreferrer">Grafana Live</a>
                    to the frontend.</p>

                <p>JSON messages can be rendered directly - nested JSON is flattened. Example messages: <br/>
                    <code>{'{"key1": "val1", "key2": "value2"}'}</code></p>

                <p>You can post-process each message via the JavaScript language.</p>
            </>,
            natsSubjectDescription: 'the subject pattern to listen on - f.e. foo.bar.>',
            mapFnLabel: 'Message Mapping JavaScript',
            mapFnDescription: <>
                Input: <code>msg</code> contains the received message as a <a
                href="https://pkg.go.dev/github.com/nats-io/nats.go#Msg" target="_blank" rel="noreferrer">nats.Msg</a>.<br/>
                Supported Return values: A map <code>{'{k: "v"}'}</code>.
            </>,
            mapFnExamples: [
                {
                    label: 'Default script',
                    title: 'The most simple script which is used by default on the backend.',
                    value: "default" as "default"
                },
                {
                    label: 'display NATS message headers',
                    title: 'display NATS message headers in Grafana',
                    value: "headers" as "headers"
                }
            ]

        };
    }
    if (queryType === "SCRIPT") {
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

                <p>The API is basically like the Go API, but with errors transparently handled.</p>
            </>,
            mapFnLabel: 'JavaScript to send requests and/or listen to responses',
            mapFnDescription:
                <>
                    Input: <code>nc</code> the <a
                    href="https://pkg.go.dev/github.com/nats-io/nats.go#Conn" target="_blank" rel="noreferrer">nats.Conn</a> you can use
                    to
                    <a href="https://pkg.go.dev/github.com/nats-io/nats.go#Conn.Subscribe">nc.Subscribe()</a>,
                    <a href="https://pkg.go.dev/github.com/nats-io/nats.go#Conn.Request">nc.Request()</a><br/> (or any
                    other interaction).<br/>
                    Supported Return values: <a href="https://pkg.go.dev/github.com/grafana/grafana-plugin-sdk-go@v0.147.0/data#Frame"
                    target="_blank" rel="noreferrer">data.Frame</a> or an error.
                </>,
            mapFnExamples: [
                {
                    label: 'multiple requests',
                    title: 'do multiple requests and concatenate the responses',
                    value: "scripting_multipleRequests" as "scripting_multipleRequests"
                },
                {
                    label: 'request with responses',
                    title: 'a request which triggers multiple responses',
                    value: "scripting_multipleResponses" as "scripting_multipleResponses"

                }
            ]

        };
    }

    return {
        title: '',
        content: <>
        </>,
        mapFnLabel: '',
        mapFnDescription:
            <>
            </>,
        mapFnExamples: [
        ]

    };
}

export class QueryEditor extends PureComponent<Props> {
    render() {
        const query = this.props.query;

        const explanation = explanationForQueryType(query.queryType);
        return (
            <FieldSet>
                <Field label="Query Type" description="How do we interact with the NATS system">
                    <RadioButtonGroup<QueryTypes>
                        options={QueryTypeOptions}
                        value={query.queryType}
                        onChange={onQueryTypeChange(this.props, 'queryType')}
                    />
                </Field>
                <Alert title={explanation.title} severity="info">
                    {explanation.content}
                </Alert>

                {explanation.natsSubjectDescription ?
                    <Field label="NATS Subject" description={explanation.natsSubjectDescription}>
                        <Input
                            className="width-27"
                            value={query.natsSubject}
                            onChange={onChange(this.props, 'natsSubject')}
                        />
                    </Field>
                    : undefined}
                <Field label="Request Timeout">
                    <Input
                        className="width-4"
                        value={query.requestTimeout}
                        onChange={onChange(this.props, 'requestTimeout')}
                    />
                </Field>
                <Field label={explanation.mapFnLabel} style={{width: '100%'}}
                       description={explanation.mapFnDescription}>
                    <JavaScriptCodeEditorField
                        expression={query.jsFn}
                        onChange={onChangeJs(this.props, 'jsFn')}
                    />
                </Field>
                {explanation.mapFnExamples ?
                    <ButtonCascader options={explanation.mapFnExamples} onChange={(value) => onChangeJs(this.props, 'jsFn')(scripts[value[0] as SCRIPT_IDS])}>
                        Example Code
                    </ButtonCascader> : null}
            </FieldSet>
        );
    }
}
