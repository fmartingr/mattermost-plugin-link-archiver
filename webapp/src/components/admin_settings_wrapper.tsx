import manifest from 'manifest';
import React, {useState, useEffect, useCallback} from 'react';

import {Client4} from 'mattermost-redux/client';

import AdminSettings from './admin_settings';

// Helper function to make authenticated requests to plugin endpoints using Client4
const makePluginRequest = async (url: string, options: RequestInit = {}): Promise<Response> => {
    const baseUrl = Client4.getUrl() || (window as any).basename || '';
    const fullUrl = `${baseUrl}${url}`;

    // Use Client4.getOptions() to get authenticated fetch options with proper headers
    // getOptions merges the provided options with authentication headers
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
    };

    if (options.headers) {
        Object.assign(headers, options.headers);
    }

    const clientOptions = Client4.getOptions({
        method: options.method || 'GET',
        headers,
        body: options.body,
    });

    return fetch(fullUrl, clientOptions);
};

type Config = {
    mimeTypeMappings: Array<{mimeTypePattern: string; archivalTool: string}>;
    defaultArchivalTool: string;
};

// Props that Mattermost provides to custom settings
type CustomSettingProps = {
    id: string;
    value: string;
    onChange: (id: string, value: string) => void;
    setSaveNeeded: () => void;
    disabled?: boolean;
};

const AdminSettingsWrapper: React.FC<CustomSettingProps> = ({id, value, onChange, setSaveNeeded, disabled}) => {
    const [config, setConfig] = useState<Config>({
        mimeTypeMappings: [],
        defaultArchivalTool: 'do_nothing',
    });
    const [archivalTools, setArchivalTools] = useState<string[]>(['do_nothing', 'direct_download']);
    const [loading, setLoading] = useState(true);
    const [error] = useState<string | null>(null);

    // Load configuration from API (primary source) or from Mattermost value prop (fallback)
    useEffect(() => {
        const loadConfig = async () => {
            try {
                // Try to load from API first (KV store is the source of truth)
                const response = await makePluginRequest(`/plugins/${manifest.id}/api/v1/config`, {
                    method: 'GET',
                });
                if (response.ok) {
                    const data = await response.json();
                    setConfig({
                        mimeTypeMappings: data.mimeTypeMappings || [],
                        defaultArchivalTool: data.defaultArchivalTool || 'do_nothing',
                    });
                    setLoading(false);
                    return;
                }
            } catch (err) {
                // Failed to load config from API, will fallback to value prop
            }

            // Fallback to Mattermost value prop if API fails
            if (value) {
                try {
                    const parsed = JSON.parse(value);
                    if (parsed && typeof parsed === 'object') {
                        setConfig({
                            mimeTypeMappings: parsed.mimeTypeMappings || [],
                            defaultArchivalTool: parsed.defaultArchivalTool || 'do_nothing',
                        });
                    }
                } catch (err) {
                    // Failed to parse config value
                }
            }
            setLoading(false);
        };

        loadConfig();
    }, [value]);

    const fetchArchivalTools = async (): Promise<void> => {
        try {
            const response = await makePluginRequest(`/plugins/${manifest.id}/api/v1/archival-tools`, {
                method: 'GET',
            });
            if (response.ok) {
                const data = await response.json();
                const tools = data.tools || [];

                // Ensure do_nothing is always first
                const allTools = ['do_nothing', ...tools.filter((t: string) => t !== 'do_nothing')];
                setArchivalTools(allTools);
            }
        } catch (err) {
            // Silently fail - will use default tools
        }
    };

    useEffect(() => {
        fetchArchivalTools();
    }, []);

    // Auto-save when config changes via API endpoint
    const handleConfigChange = useCallback(async (newConfig: Config) => {
        setConfig(newConfig);

        // Auto-save to backend via API endpoint
        try {
            const response = await makePluginRequest(`/plugins/${manifest.id}/api/v1/config`, {
                method: 'POST',
                body: JSON.stringify(newConfig),
            });
            if (!response.ok) {
                // Failed to auto-save configuration
            }
        } catch (err) {
            // Failed to auto-save configuration
        }

        // Also update Mattermost's setting value for consistency
        const serialized = JSON.stringify(newConfig);
        onChange(id, serialized);
        setSaveNeeded();
    }, [id, onChange, setSaveNeeded]);

    if (loading) {
        return (
            <div style={{padding: '20px'}}>
                <div>{'Loading configuration...'}</div>
            </div>
        );
    }

    if (error) {
        return (
            <div style={{padding: '20px'}}>
                <div style={{color: '#d32f2f', marginBottom: '10px', fontSize: '14px'}}>
                    {`Error: ${error}`}
                </div>
            </div>
        );
    }

    return (
        <AdminSettings
            config={config}
            setConfig={handleConfigChange}
            archivalTools={archivalTools}
            disabled={disabled}
        />
    );
};

export default AdminSettingsWrapper;
