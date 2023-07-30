import React, { PureComponent } from 'react';
import {
  onUpdateDatasourceJsonDataOption,
  DataSourcePluginOptionsEditorProps,
  onUpdateDatasourceSecureJsonDataOption, onUpdateDatasourceJsonDataOptionSelect
} from '@grafana/data';
import {Select, InlineField, Input, TextArea, FieldSet} from '@grafana/ui';
import {AuthenticationOptions, MyDataSourceOptions, MySecureJsonData} from '../types';

// https://github.com/grafana/grafana/tree/main/packages/grafana-ui/src/components

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

interface State {}

export class ConfigEditor extends PureComponent<Props, State> {
  render() {
    const { options } = this.props;
    const { jsonData } = options;
    const secureJsonData = (options.secureJsonData || ({} as MySecureJsonData));

    return (
      <FieldSet>
        <InlineField label="NATS Server URL" tooltip="demo.nats.io:4222 or tls://demo.nats.io:4222">
          <Input
              className="width-27"
              value={jsonData.natsUrl}
              placeholder="demo.nats.io:4222 or tls://demo.nats.io:4222"
              onChange={onUpdateDatasourceJsonDataOption(this.props, 'natsUrl')}
          />
        </InlineField>
        <InlineField label="Authentication Mode" tooltip="How do you authenticate with the server">
          <Select
              options={AuthenticationOptions}
              value={jsonData.authentication}
              onChange={onUpdateDatasourceJsonDataOptionSelect(this.props, 'authentication')}
          />
        </InlineField>

        {jsonData.authentication === "NKEY" ?
          <>
            <InlineField label="Public NKEY" tooltip="U...">
              <Input
                  className="width-27"
                  value={jsonData.nkey}
                  placeholder="U..."
                  onChange={onUpdateDatasourceJsonDataOption(this.props, 'nkey')}
              />
            </InlineField>
            <InlineField label="Private NKEY Seed" tooltip="SU...">
              <Input
                  type="password"
                  className="width-27"
                  value={secureJsonData.nkeySeed}
                  placeholder="SU..."
                  onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'nkeySeed')}
              />
            </InlineField>
          </>
          : null}

        {jsonData.authentication === "USERPASS" ?
            <>
                <InlineField label="Username">
                  <Input
                      className="width-27"
                      value={jsonData.username}
                      onChange={onUpdateDatasourceJsonDataOption(this.props, 'username')}
                  />
                </InlineField>
                <InlineField label="Password">
                  <Input
                      type="password"
                      className="width-27"
                      value={secureJsonData.password}
                      onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'password')}
                  />
                </InlineField>
            </>
            : null}

        {jsonData.authentication === "JWT" ?
            <>
                <InlineField label="JWT">
                  <TextArea
                      className="width-27"
                      value={secureJsonData.jwt}
                      onChange={onUpdateDatasourceSecureJsonDataOption(this.props, 'jwt')}
                  />
                </InlineField>
            </>
            : null}
      </FieldSet>
    );
  }
}
