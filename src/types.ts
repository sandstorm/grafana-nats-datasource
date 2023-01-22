import {DataQuery, DataSourceJsonData} from '@grafana/data';

export interface MyQuery extends DataQuery {
    queryText?: string;
    constant: number;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
    constant: 6.5,
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

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend.
 * In the UI, they are write-only.
 */
export interface MySecureJsonData {
    nkeySeed?: string;
    password?: string;
    jwt?: string;
}
