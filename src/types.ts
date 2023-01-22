import {DataQuery, DataSourceJsonData} from '@grafana/data';
// These need to be synced with types.go

export interface MyQuery extends DataQuery {
    queryType: "REQUEST_REPLY";
    natsSubject: string;
    requestTimeout: string;
    requestData: string;
    jqExpression: string;
}

export const QueryTypeOptions = [
    {
        label: "Request/Reply",
        value: "REQUEST_REPLY"
    }
];

export const DEFAULT_QUERY: Partial<MyQuery> = {
    queryType: "REQUEST_REPLY",
    requestTimeout: "5s"
};


type AuthenticationModes = "NONE" | "NKEY" | "USERPASS" | "JWT";

export const AuthenticationOptions = [
    {
        label: "no authentication",
        value: "NONE"
    }, {
        label: "NKEY based authentication",
        value: "NKEY"
    }, {
        label: "User / password authentication",
        value: "USERPASS"
    }, {
        label: "JWT based authentication",
        value: "JWT"
    }
];

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
    natsUrl?: string;
    authentication: AuthenticationModes;
    nkey?: string;
    username?: string;
}

// These need to be synced with types.go

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend.
 * In the UI, they are write-only.
 */
export interface MySecureJsonData {
    nkeySeed?: string;
    password?: string;
    jwt?: string;
}
