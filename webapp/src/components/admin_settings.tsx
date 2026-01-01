import React, {useState, useEffect} from 'react';

type MimeTypeMapping = {
    mimeTypePattern: string;
    archivalTool: string;
};

type Config = {
    mimeTypeMappings: MimeTypeMapping[];
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
        setLocalConfig(config);
    }, [config]);

    const handleAddMapping = () => {
        const defaultTool = archivalTools.length > 0 ? archivalTools[0] : 'do_nothing';
        const newConfig = {
            ...localConfig,
            mimeTypeMappings: [
                ...localConfig.mimeTypeMappings,
                {mimeTypePattern: '', archivalTool: defaultTool},
            ],
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    const handleRemoveMapping = (index: number) => {
        const newMappings = [...localConfig.mimeTypeMappings];
        newMappings.splice(index, 1);
        const newConfig = {
            ...localConfig,
            mimeTypeMappings: newMappings,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    const handleUpdateMapping = (index: number, field: keyof MimeTypeMapping, value: string) => {
        const newMappings = [...localConfig.mimeTypeMappings];
        newMappings[index] = {
            ...newMappings[index],
            [field]: value,
        };
        const newConfig = {
            ...localConfig,
            mimeTypeMappings: newMappings,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    const handleDefaultToolChange = (value: string) => {
        const newConfig = {
            ...localConfig,
            defaultArchivalTool: value,
        };
        setLocalConfig(newConfig);
        setConfig(newConfig);
    };

    return (
        <div style={styles.container}>
            {/* MIME Type Mappings Section */}
            <div style={styles.section}>
                <div style={styles.sectionTitle}>{'MIME Type Mappings'}</div>
                <div style={styles.formGroup}>
                    <div style={styles.helpText}>
                        {'Configure which archival tool to use for different MIME types. Use wildcards like "image/*" to match all image types.'}
                    </div>

                    {localConfig.mimeTypeMappings.length === 0 ? (
                        <div style={styles.emptyState}>
                            <div style={styles.emptyStateText}>
                                {'No MIME type mappings configured. Click "Add Mapping" to create one.'}
                            </div>
                            <button
                                type='button'
                                onClick={handleAddMapping}
                                disabled={disabled}
                                style={{
                                    ...styles.button,
                                    ...styles.buttonSecondary,
                                    ...(disabled ? styles.buttonDisabled : {}),
                                }}
                            >
                                {'Add Mapping'}
                            </button>
                        </div>
                    ) : (
                        <>
                            <table style={styles.table}>
                                <thead>
                                    <tr>
                                        <th style={styles.tableHeader}>{'MIME Type Pattern'}</th>
                                        <th style={styles.tableHeader}>{'Archival Tool'}</th>
                                        <th style={styles.tableHeader}>{'Actions'}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {localConfig.mimeTypeMappings.map((mapping, index) => (
                                        <tr key={index}>
                                            <td style={styles.tableCell}>
                                                <input
                                                    type='text'
                                                    style={styles.tableInput}
                                                    value={mapping.mimeTypePattern}
                                                    onChange={(e) => handleUpdateMapping(index, 'mimeTypePattern', e.target.value)}
                                                    placeholder='e.g., application/pdf, image/*'
                                                    disabled={disabled}
                                                />
                                            </td>
                                            <td style={styles.tableCell}>
                                                <select
                                                    style={styles.tableSelect}
                                                    value={mapping.archivalTool}
                                                    onChange={(e) => handleUpdateMapping(index, 'archivalTool', e.target.value)}
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
                                                <button
                                                    type='button'
                                                    onClick={() => handleRemoveMapping(index)}
                                                    disabled={disabled}
                                                    style={{
                                                        ...styles.button,
                                                        ...styles.buttonDanger,
                                                        ...(disabled ? styles.buttonDisabled : {}),
                                                    }}
                                                >
                                                    {'Remove'}
                                                </button>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>

                            <button
                                type='button'
                                onClick={handleAddMapping}
                                disabled={disabled}
                                style={{
                                    ...styles.button,
                                    ...styles.buttonSecondary,
                                    ...(disabled ? styles.buttonDisabled : {}),
                                }}
                            >
                                {'Add Mapping'}
                            </button>
                        </>
                    )}
                </div>
            </div>

            {/* Default Archival Tool Section */}
            <div style={styles.section}>
                <div style={styles.sectionTitle}>{'Default Archival Tool'}</div>
                <div style={styles.formGroup}>
                    <label style={styles.label}>
                        {'Default Archival Tool'}
                    </label>
                    <select
                        style={{
                            ...styles.select,
                            ...(disabled ? {opacity: 0.6, cursor: 'not-allowed'} : {}),
                        }}
                        value={localConfig.defaultArchivalTool}
                        onChange={(e) => handleDefaultToolChange(e.target.value)}
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
                    <div style={styles.helpText}>
                        {'The default archival tool to use when no MIME type mapping matches. This is a required setting.'}
                    </div>
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
