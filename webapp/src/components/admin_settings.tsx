import React, {useState, useEffect} from 'react';

type ArchivalRule = {
    kind: 'hostname' | 'mimetype';
    pattern: string;
    archivalTool: string;
};

type Config = {
    archivalRules: ArchivalRule[];
    defaultArchivalTool: string;
};

type Props = {
    config: Config;
    setConfig: (config: Config) => void;
    archivalTools: string[];
    disabled?: boolean;
};

// Mattermost admin console styling
const styles = {
    container: {
        padding: '20px',
        maxWidth: '100%',
    },
    section: {
        marginBottom: '30px',
    },
    sectionTitle: {
        fontSize: '16px',
        fontWeight: 600,
        color: '#3d3d3d',
        marginBottom: '12px',
        paddingBottom: '8px',
        borderBottom: '1px solid #e0e0e0',
    },
    formGroup: {
        marginBottom: '20px',
    },
    label: {
        display: 'block',
        fontSize: '14px',
        fontWeight: 600,
        color: '#3d3d3d',
        marginBottom: '8px',
    },
    helpText: {
        fontSize: '12px',
        color: '#666',
        marginTop: '6px',
        lineHeight: '1.5',
    },
    table: {
        width: '100%',
        borderCollapse: 'collapse' as const,
        marginTop: '12px',
        marginBottom: '12px',
    },
    tableHeader: {
        backgroundColor: '#f5f5f5',
        borderBottom: '2px solid #ddd',
        padding: '12px',
        textAlign: 'left' as const,
        fontSize: '13px',
        fontWeight: 600,
        color: '#3d3d3d',
    },
    tableCell: {
        padding: '12px',
        borderBottom: '1px solid #e0e0e0',
        fontSize: '14px',
    },
    tableInput: {
        width: '100%',
        padding: '6px 10px',
        fontSize: '14px',
        border: '1px solid #ddd',
        borderRadius: '4px',
        boxSizing: 'border-box' as const,
    },
    tableSelect: {
        width: '100%',
        padding: '6px 10px',
        fontSize: '14px',
        border: '1px solid #ddd',
        borderRadius: '4px',
        backgroundColor: '#fff',
        boxSizing: 'border-box' as const,
    },
    button: {
        padding: '8px 16px',
        fontSize: '14px',
        fontWeight: 600,
        borderRadius: '4px',
        border: 'none',
        cursor: 'pointer',
        transition: 'background-color 0.2s',
    },
    buttonSecondary: {
        backgroundColor: '#f5f5f5',
        color: '#3d3d3d',
        border: '1px solid #ddd',
    },
    buttonDanger: {
        backgroundColor: '#d32f2f',
        color: '#fff',
    },
    buttonDisabled: {
        opacity: 0.6,
        cursor: 'not-allowed',
    },
    emptyState: {
        padding: '40px 20px',
        textAlign: 'center' as const,
        color: '#666',
    },
    emptyStateText: {
        fontSize: '14px',
        marginBottom: '16px',
    },
    select: {
        width: '100%',
        maxWidth: '400px',
        padding: '8px 12px',
        fontSize: '14px',
        border: '1px solid #ddd',
        borderRadius: '4px',
        backgroundColor: '#fff',
        boxSizing: 'border-box' as const,
    },
};

const AdminSettings: React.FC<Props> = ({config, setConfig, archivalTools, disabled}) => {
    const [localConfig, setLocalConfig] = useState<Config>(config);

    useEffect(() => {
        // Ensure there's always a default rule at the end
        const rules = [...config.archivalRules];
        if (rules.length === 0 || rules[rules.length - 1].pattern !== '') {
            // Add or ensure default rule exists
            const defaultTool = config.defaultArchivalTool || 'do_nothing';
            if (rules.length === 0 || rules[rules.length - 1].pattern !== '') {
                rules.push({
                    kind: 'mimetype',
                    pattern: '', // Empty pattern means it's the default
                    archivalTool: defaultTool,
                });
            } else {
                // Update existing default rule
                rules[rules.length - 1].archivalTool = defaultTool;
            }
        }
        setLocalConfig({
            ...config,
            archivalRules: rules,
        });
    }, [config]);

    const handleAddRule = () => {
        const defaultTool = archivalTools.length > 0 ? archivalTools[0] : 'do_nothing';

        // Add rule before the last one (which is the default)
        const rules = [...localConfig.archivalRules];
        const defaultRule = rules.pop() || {kind: 'mimetype' as const, pattern: '', archivalTool: 'do_nothing'};
        rules.push({kind: 'mimetype', pattern: '', archivalTool: defaultTool});
        rules.push(defaultRule); // Put default back at the end
        const newConfig = {
            ...localConfig,
            archivalRules: rules,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    const handleRemoveRule = (index: number) => {
        // Don't allow removing the last rule (default)
        if (index === localConfig.archivalRules.length - 1) {
            return;
        }
        const newRules = [...localConfig.archivalRules];
        newRules.splice(index, 1);
        const newConfig = {
            ...localConfig,
            archivalRules: newRules,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    const handleUpdateRule = (index: number, field: keyof ArchivalRule, value: string) => {
        const newRules = [...localConfig.archivalRules];
        newRules[index] = {
            ...newRules[index],
            [field]: value,
        };

        // If updating the default rule's tool, also update defaultArchivalTool
        const isDefault = index === newRules.length - 1;
        const newConfig = {
            ...localConfig,
            archivalRules: newRules,
            defaultArchivalTool: isDefault && field === 'archivalTool' ? value : localConfig.defaultArchivalTool,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    const handleMoveRule = (index: number, direction: 'up' | 'down') => {
        // Don't allow moving the last rule (default)
        const lastIndex = localConfig.archivalRules.length - 1;
        if (index === lastIndex) {
            return;
        }
        const newRules = [...localConfig.archivalRules];
        if (direction === 'up' && index > 0) {
            [newRules[index - 1], newRules[index]] = [newRules[index], newRules[index - 1]];
        } else if (direction === 'down' && index < lastIndex - 1) {
            // Can't move down if it would become the last rule
            [newRules[index], newRules[index + 1]] = [newRules[index + 1], newRules[index]];
        }
        const newConfig = {
            ...localConfig,
            archivalRules: newRules,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    return (
        <div style={styles.container}>
            {/* Archival Rules Section */}
            <div style={styles.section}>
                <div style={styles.sectionTitle}>{'Archival Rules'}</div>
                <div style={styles.formGroup}>
                    <div style={styles.helpText}>
                        {'Configure archival rules that match on hostname or MIME type patterns. Rules are evaluated in order, and the first matching rule determines which archival tool to use. The last rule is the default (always matches) and cannot be removed or reordered. Use wildcards like "*.example.com" for hostnames or "image/*" for MIME types.'}
                    </div>

                    <table style={styles.table}>
                        <thead>
                            <tr>
                                <th style={styles.tableHeader}>{'Order'}</th>
                                <th style={styles.tableHeader}>{'Kind'}</th>
                                <th style={styles.tableHeader}>{'Pattern'}</th>
                                <th style={styles.tableHeader}>{'Archival Tool'}</th>
                                <th style={styles.tableHeader}>{'Actions'}</th>
                            </tr>
                        </thead>
                        <tbody>
                            {localConfig.archivalRules.map((rule, index) => {
                                const isDefault = index === localConfig.archivalRules.length - 1;
                                const hasPattern = rule.pattern && rule.pattern.trim() !== '';
                                let placeholder = 'e.g., image/*';
                                if (rule.kind === 'hostname') {
                                    placeholder = 'e.g., *.example.com';
                                } else if (isDefault) {
                                    placeholder = 'Default (always matches)';
                                }
                                const rowStyle = isDefault ? {backgroundColor: '#f9f9f9'} : {};
                                return (
                                    <tr
                                        key={index}
                                        style={rowStyle}
                                    >
                                        <td style={styles.tableCell}>
                                            {isDefault ? (
                                                <span style={{color: '#666', fontSize: '12px'}}>{'Default'}</span>
                                            ) : (
                                                <div style={{display: 'flex', flexDirection: 'column', gap: '4px'}}>
                                                    <button
                                                        type='button'
                                                        onClick={() => handleMoveRule(index, 'up')}
                                                        disabled={disabled || index === 0}
                                                        style={{
                                                            ...styles.button,
                                                            ...styles.buttonSecondary,
                                                            padding: '4px 8px',
                                                            fontSize: '12px',
                                                            ...(disabled || index === 0 ? styles.buttonDisabled : {}),
                                                        }}
                                                        title='Move up'
                                                    >
                                                        {'↑'}
                                                    </button>
                                                    <button
                                                        type='button'
                                                        onClick={() => handleMoveRule(index, 'down')}
                                                        disabled={disabled || index === localConfig.archivalRules.length - 2}
                                                        style={{
                                                            ...styles.button,
                                                            ...styles.buttonSecondary,
                                                            padding: '4px 8px',
                                                            fontSize: '12px',
                                                            ...(disabled || index === localConfig.archivalRules.length - 2 ? styles.buttonDisabled : {}),
                                                        }}
                                                        title='Move down'
                                                    >
                                                        {'↓'}
                                                    </button>
                                                </div>
                                            )}
                                        </td>
                                        {!isDefault && (
                                            <>
                                                <td style={styles.tableCell}>
                                                    <select
                                                        style={styles.tableSelect}
                                                        value={rule.kind}
                                                        onChange={(e) => handleUpdateRule(index, 'kind', e.target.value as 'hostname' | 'mimetype')}
                                                        disabled={disabled}
                                                    >
                                                        <option value='hostname'>{'Hostname'}</option>
                                                        <option value='mimetype'>{'MIME Type'}</option>
                                                    </select>
                                                </td>
                                                <td style={styles.tableCell}>
                                                    <input
                                                        type='text'
                                                        style={{
                                                            ...styles.tableInput,
                                                            ...(hasPattern === false ? {borderColor: '#d32f2f'} : {}),
                                                        }}
                                                        value={rule.pattern}
                                                        onChange={(e) => handleUpdateRule(index, 'pattern', e.target.value)}
                                                        placeholder={placeholder}
                                                        disabled={disabled}
                                                    />
                                                </td>
                                            </>
                                        )}
                                        {isDefault && (
                                            <td
                                                colSpan={2}
                                                style={styles.tableCell}
                                            >
                                                <span style={{color: '#666', fontSize: '12px', fontStyle: 'italic'}}>{'Default rule (always matches)'}</span>
                                            </td>
                                        )}
                                        <td style={styles.tableCell}>
                                            <select
                                                style={styles.tableSelect}
                                                value={rule.archivalTool}
                                                onChange={(e) => handleUpdateRule(index, 'archivalTool', e.target.value)}
                                                disabled={disabled}
                                            >
                                                {archivalTools.map((tool) => (
                                                    <option
                                                        key={tool}
                                                        value={tool}
                                                    >
                                                        {formatToolName(tool)}
                                                    </option>
                                                ))}
                                            </select>
                                        </td>
                                        <td style={styles.tableCell}>
                                            {isDefault ? (
                                                <span style={{color: '#666', fontSize: '12px'}}>{'Default'}</span>
                                            ) : (
                                                <button
                                                    type='button'
                                                    onClick={() => handleRemoveRule(index)}
                                                    disabled={disabled}
                                                    style={{
                                                        ...styles.button,
                                                        ...styles.buttonDanger,
                                                        ...(disabled ? styles.buttonDisabled : {}),
                                                    }}
                                                >
                                                    {'Remove'}
                                                </button>
                                            )}
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>

                    <button
                        type='button'
                        onClick={handleAddRule}
                        disabled={disabled}
                        style={{
                            ...styles.button,
                            ...styles.buttonSecondary,
                            ...(disabled ? styles.buttonDisabled : {}),
                        }}
                    >
                        {'Add Rule'}
                    </button>
                </div>
            </div>
        </div>
    );
};

// formatToolName converts a tool name like "direct_download" to "Direct Download" or "do_nothing" to "Do Nothing"
const formatToolName = (toolName: string): string => {
    if (toolName === 'do_nothing') {
        return 'Do Nothing';
    }
    return toolName.
        split('_').
        map((word) => word.charAt(0).toUpperCase() + word.slice(1)).
        join(' ');
};

export default AdminSettings;
