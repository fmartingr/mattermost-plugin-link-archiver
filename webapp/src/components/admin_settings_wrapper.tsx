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

type ArchivalRule = {
    kind: 'hostname' | 'mimetype';
    pattern: string;
    archivalTool: string;
};

type Config = {
    archivalRules: ArchivalRule[];
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
        archivalRules: [],
        defaultArchivalTool: 'do_nothing',
    });
    const [archivalTools, setArchivalTools] = useState<string[]>(['do_nothing', 'direct_download']);
    const [loading, setLoading] = useState(true);
    const [error] = useState<string | null>(null);

    // Helper function to migrate old format rules to new format
    const migrateRules = (rules: any[]): ArchivalRule[] => {
        return rules.map((rule: any) => {
            if (rule.kind && rule.pattern) {
                // New format
                return rule;
            } else if (rule.hostnamePattern) {
                // Old format with hostname
                return {
                    kind: 'hostname',
                    pattern: rule.hostnamePattern,
                    archivalTool: rule.archivalTool,
                };
            } else if (rule.mimeTypePattern) {
                // Old format with mimetype
                return {
                    kind: 'mimetype',
                    pattern: rule.mimeTypePattern,
                    archivalTool: rule.archivalTool,
                };
            }
            // Fallback - shouldn't happen
            return {
                kind: 'mimetype',
                pattern: '',
                archivalTool: rule.archivalTool || 'do_nothing',
            };
        });
    };

    // Load configuration only once on mount from Mattermost value prop
    // This prevents reloading during edits
    useEffect(() => {
        const loadConfig = () => {
            if (value) {
                try {
                    const parsed = JSON.parse(value);
                    if (parsed && typeof parsed === 'object') {
                        // Handle migration from old format
                        let archivalRules: ArchivalRule[] = [];
                        if (parsed.archivalRules && Array.isArray(parsed.archivalRules)) {
                            archivalRules = migrateRules(parsed.archivalRules);
                        } else if (parsed.mimeTypeMappings && Array.isArray(parsed.mimeTypeMappings)) {
                            // Migrate old format to new format
                            archivalRules = parsed.mimeTypeMappings.map((mapping: {mimeTypePattern: string; archivalTool: string}) => ({
                                kind: 'mimetype',
                                pattern: mapping.mimeTypePattern,
                                archivalTool: mapping.archivalTool,
                            }));
                        }

                        // Ensure there's always a default rule at the end (empty pattern)
                        const defaultTool = parsed.defaultArchivalTool || 'do_nothing';
                        if (archivalRules.length === 0 || archivalRules[archivalRules.length - 1].pattern !== '') {
                            // Add default rule if it doesn't exist
                            archivalRules.push({
                                kind: 'mimetype',
                                pattern: '',
                                archivalTool: defaultTool,
                            });
                        } else {
                            // Update existing default rule
                            archivalRules[archivalRules.length - 1].archivalTool = defaultTool;
                        }

                        setConfig({
                            archivalRules,
                            defaultArchivalTool: defaultTool,
                        });
                    }
                } catch (err) {
                    // Failed to parse config value
                }
            }
            setLoading(false);
        };

        loadConfig();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []); // Only run once on mount

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

    // Update local state and mark as needing save (no auto-save to API)
    // The system admin "Save" button will trigger OnConfigurationChange which saves to KV store
    const handleConfigChange = useCallback((newConfig: Config) => {
        setConfig(newConfig);

        // Update Mattermost's setting value and mark as needing save
        // This will be saved when the user clicks the system admin "Save" button
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
