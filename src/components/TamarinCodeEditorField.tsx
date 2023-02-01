import type * as monacoType from 'monaco-editor/esm/vs/editor/editor.api';
import React, {useCallback, useRef} from 'react';

import {CodeEditor, Monaco} from '@grafana/ui';
// inspired by https://github.com/grafana/grafana/blob/78184f37c444bd8c36437498bde365a6a81bb71d/public/app/plugins/datasource/cloudwatch/components/MathExpressionQueryField.tsx


export interface Props {
    onChange: (query: string) => void;
    expression: string;
    //datasource: CloudWatchDatasource;
}


// extra libraries
const libSource = `
declare class Facts {
    /**
     * Returns the next fact
     */
    static next():string
`;
const libUri = 'ts:filename/facts.d.ts';


/*const TRIGGER_SUGGEST = {
    id: 'editor.action.triggerSuggest',
    title: '',
};*/

export function TamarinCodeEditorField({expression: expression, onChange}: React.PropsWithChildren<Props>) {
    const containerRef = useRef<HTMLDivElement>(null);
    const onEditorMount = useCallback(
        (editor: monacoType.editor.IStandaloneCodeEditor, monaco: Monaco) => {
            //editor.onDidFocusEditorText(() => editor.trigger(TRIGGER_SUGGEST.id, TRIGGER_SUGGEST.id, {}));
            editor.addCommand(monaco.KeyMod.Shift | monaco.KeyCode.Enter, () => {
                const text = editor.getValue();
                onChange(text);
            });

            // auto resizes the editor to be the height of the content it holds
            // this code comes from the Prometheus query editor.
            // We may wish to consider abstracting it into the grafana/ui repo in the future
            const updateElementHeight = () => {
                const containerDiv = containerRef.current;
                if (containerDiv !== null && editor.getContentHeight() < 200) {
                    const pixelHeight = Math.max(100, editor.getContentHeight());
                    containerDiv.style.height = `${pixelHeight}px`;
                    containerDiv.style.width = '100%';
                    const pixelWidth = containerDiv.clientWidth;
                    editor.layout({width: pixelWidth, height: pixelHeight});
                }
            };

            editor.onDidContentSizeChange(updateElementHeight);
            updateElementHeight();
        },
        [onChange]
    );

    return (
        <div ref={containerRef} style={{width: '100%'}}>
            <CodeEditor
                monacoOptions={{
                    // without this setting, the auto-resize functionality causes an infinite loop, don't remove it!
                    scrollBeyondLastLine: false,

                    // These additional options are style focused and are a subset of those in the query editor in Prometheus
                    fontSize: 14,
                    lineNumbers: "on",
                    renderLineHighlight: 'none',
                    scrollbar: {
                        vertical: 'hidden',
                        horizontal: 'hidden',
                    },
                    suggestFontSize: 12,
                    wordWrap: 'on',
                    padding: {
                        top: 6,
                    },
                }}

                language="javascript"
                value={expression}
                onBlur={(value) => {
                    if (value !== expression) {
                        onChange(value);
                    }
                }}
                onBeforeEditorMount={(monaco: Monaco) => {
                    monaco.languages.typescript.javascriptDefaults.addExtraLib(libSource, libUri);
                    // When resolving definitions and references, the editor will try to use created models.
                    // Creating a model for the library allows "peek definition/references" commands to work with the library.
                    monaco.editor.createModel(libSource, 'typescript', monaco.Uri.parse(libUri));

                    // see https://github.com/microsoft/monaco-editor/issues/1661#issuecomment-777289233
                    monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions({
                        noSemanticValidation: false,
                        noSyntaxValidation: false,
                        diagnosticCodesToIgnore: [/* top-level return */ 1108]
                    });

                }}

                onEditorDidMount={onEditorMount}
            />
        </div>
    );
}
