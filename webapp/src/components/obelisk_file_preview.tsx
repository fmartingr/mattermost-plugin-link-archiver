import React, {useState, useEffect, useRef, useMemo} from 'react';

import type {FileInfo} from '@mattermost/types/files';
import type {Post} from '@mattermost/types/posts';

type Props = {
    fileInfo: FileInfo;
    post: Post;
};

const ObeliskFilePreview: React.FC<Props> = (props) => {
    // Mattermost passes fileInfo (singular) and post as props
    // Try multiple ways to get fileInfo in case Mattermost passes it differently
    const fileInfo = useMemo(() => {
        // Method 1: Direct prop access (expected way)
        if (props && 'fileInfo' in props && props.fileInfo) {
            return props.fileInfo;
        }

        // Method 2: Check for fileInfos (plural) as fallback
        if (props && 'fileInfos' in props) {
            // eslint-disable-next-line react/prop-types
            const fileInfos = (props as any).fileInfos;
            // eslint-disable-next-line react/prop-types
            if (Array.isArray(fileInfos) && fileInfos.length > 0) {
                return fileInfos[0];
            } else if (fileInfos && typeof fileInfos === 'object' && !Array.isArray(fileInfos)) {
                return fileInfos as FileInfo;
            }
        }

        // Method 3: Props might be the fileInfo directly
        if (props && typeof props === 'object' && !Array.isArray(props)) {
            if ('id' in props && 'name' in props) {
                return props as unknown as FileInfo;
            }
        }

        return null;
    }, [props]);

    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [blobUrl, setBlobUrl] = useState<string | null>(null);
    const iframeRef = useRef<HTMLIFrameElement>(null);

    useEffect(() => {
        const loadFile = async () => {
            if (!fileInfo) {
                setError('No file to preview');
                setLoading(false);
                return;
            }

            // Check if it's an obelisk-archived file (by extension or name pattern)
            const isObeliskFile = fileInfo.name?.endsWith('.obelisk.html') === true;

            if (!isObeliskFile && !(fileInfo.mime_type?.startsWith('text/html') || fileInfo.mime_type?.startsWith('application/xhtml'))) {
                setError('File is not an obelisk-archived file');
                setLoading(false);
                return;
            }

            try {
                // Get the file URL - Mattermost provides file URLs through the API
                // Use the file link if available, otherwise construct it
                const baseUrl = (window as any).basename || '';
                let fileUrl: string;

                if (fileInfo.link) {
                    // Use the provided link if available
                    fileUrl = fileInfo.link.startsWith('http') ? fileInfo.link : `${baseUrl}${fileInfo.link}`;
                } else {
                    // Fallback to constructing the URL
                    fileUrl = `${baseUrl}/api/v4/files/${fileInfo.id}`;
                }

                // Fetch the file content
                const response = await fetch(fileUrl, {
                    credentials: 'include',
                    headers: {
                        'Content-Type': 'text/html',
                    },
                });

                if (!response.ok) {
                    throw new Error(`Failed to load file: ${response.status} ${response.statusText}`);
                }

                const text = await response.text();
                if (!text || text.trim().length === 0) {
                    throw new Error('File is empty');
                }

                // Create a blob URL for the HTML content to display in iframe
                const blob = new Blob([text], {type: 'text/html'});
                const url = URL.createObjectURL(blob);
                setBlobUrl(url);
            } catch (err) {
                setError(err instanceof Error ? err.message : 'Failed to load file');
            } finally {
                setLoading(false);
            }
        };

        loadFile();
    }, [fileInfo]);

    // Cleanup blob URL on unmount or when blobUrl changes
    useEffect(() => {
        return () => {
            if (blobUrl) {
                URL.revokeObjectURL(blobUrl);
            }
        };
    }, [blobUrl]);

    if (loading) {
        return (
            <div style={{padding: '20px', textAlign: 'center'}}>
                <div>{'Loading archived page...'}</div>
            </div>
        );
    }

    if (error) {
        return (
            <div style={{padding: '20px', color: '#d32f2f'}}>
                <div>{`Error: ${error}`}</div>
            </div>
        );
    }

    if (!blobUrl) {
        return (
            <div style={{padding: '20px'}}>
                <div>{'No content to display'}</div>
            </div>
        );
    }

    return (
        <div
            style={{
                width: '100%',
                height: 'calc(100vh - 200px)', // Subtract space for Mattermost UI (header, buttons, etc.)
                minHeight: '400px',
                maxHeight: '90vh',
                border: '1px solid #ddd',
                borderRadius: '4px',
                overflow: 'hidden',
                display: 'flex',
                flexDirection: 'column',
            }}
        >
            <iframe
                ref={iframeRef}
                src={blobUrl}
                style={{
                    width: '100%',
                    height: '100%',
                    border: 'none',
                    flex: '1 1 auto',
                }}
                title='Archived page preview'
                sandbox='allow-same-origin allow-scripts'
            />
        </div>
    );
};

export default ObeliskFilePreview;

