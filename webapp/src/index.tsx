// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import manifest from 'manifest';

import type {FileInfo} from '@mattermost/types/files';

import type {PluginRegistry} from 'types/mattermost-webapp';

import AdminSettingsWrapper from './components/admin_settings_wrapper';
import ObeliskFilePreview from './components/obelisk_file_preview';

export default class Plugin {
    public async initialize(registry: PluginRegistry) {
        // Register admin console custom setting for MIME type mappings configuration
        // The key must match the key in plugin.json settings_schema.settings
        registry.registerAdminConsoleCustomSetting('MimeTypeMappings', AdminSettingsWrapper, {showTitle: false});

        // Register file preview component for obelisk-archived HTML files
        registry.registerFilePreviewComponent(
            (fileInfos: FileInfo[] | FileInfo) => {
                // Get the first file (we only handle one archived file per post)
                let fileInfo: FileInfo | null = null;

                if (Array.isArray(fileInfos) && fileInfos.length > 0) {
                    fileInfo = fileInfos[0];
                } else if (fileInfos && typeof fileInfos === 'object' && fileInfos !== null && !Array.isArray(fileInfos)) {
                    fileInfo = fileInfos as FileInfo;
                }

                if (!fileInfo) {
                    return false;
                }

                // Check by filename extension - must have .obelisk.html
                const hasObeliskExtension = fileInfo.name?.endsWith('.obelisk.html') === true;
                if (hasObeliskExtension) {
                    return true;
                }

                // Also check if MIME type is text/html and filename contains 'obelisk' or 'archived'
                const isHTMLMimeType = fileInfo.mime_type?.startsWith('text/html') === true ||
                    fileInfo.mime_type?.startsWith('application/xhtml') === true;
                const hasArchiveIndicator = fileInfo.name?.toLowerCase().includes('obelisk') === true ||
                    fileInfo.name?.toLowerCase().includes('archived') === true;

                return isHTMLMimeType && hasArchiveIndicator;
            },
            ObeliskFilePreview,
        );
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
