import { FileItem } from "@/lib/api";

export const FOLDER_MIME_TYPE = "application/vnd.google-apps.folder";

export interface PartitionedFiles {
  folders: FileItem[];
  notes: FileItem[];
}

/**
 * Split a mixed file listing into folders (name-ascending) and notes
 * (modifiedTime-descending). Pure — safe to unit test directly.
 */
export function partitionFilesByKind(items: FileItem[]): PartitionedFiles {
  const folders = items
    .filter((item) => item.mimeType === FOLDER_MIME_TYPE)
    .sort((a, b) => a.name.localeCompare(b.name));

  const notes = items
    .filter((item) => item.mimeType !== FOLDER_MIME_TYPE)
    .sort(
      (a, b) =>
        new Date(b.modifiedTime).getTime() - new Date(a.modifiedTime).getTime(),
    );

  return { folders, notes };
}
