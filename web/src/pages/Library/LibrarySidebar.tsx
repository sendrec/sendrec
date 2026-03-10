import { Link } from "react-router-dom";
import type { Folder, Tag } from "../../types/video";
import type { LimitsResponse } from "../../types/limits";

interface Playlist {
  id: string;
  title: string;
  videoCount: number;
}

interface LibrarySidebarProps {
  folders: Folder[];
  tags: Tag[];
  playlists: Playlist[];
  limits: LimitsResponse | null;
  activeFilter: string;
  onFilterChange: (filter: string) => void;
  creatingFolder: boolean;
  onSetCreatingFolder: (value: boolean) => void;
  newFolderName: string;
  onSetNewFolderName: (value: string) => void;
  onCreateFolder: () => void;
  creatingTag: boolean;
  onSetCreatingTag: (value: boolean) => void;
  newTagName: string;
  onSetNewTagName: (value: string) => void;
  newTagColor: string;
  onSetNewTagColor: (value: string) => void;
  onCreateTag: () => void;
  editingSidebarId: string | null;
  editingSidebarName: string;
  onSetEditingSidebarId: (value: string | null) => void;
  onSetEditingSidebarName: (value: string) => void;
  onRenameSidebarItem: (type: "folder" | "tag", id: string) => void;
  onDeleteSidebarItem: (type: "folder" | "tag", id: string) => void;
  sidebarMenuId: string | null;
  onSetSidebarMenuId: (value: string | null) => void;
}

export function LibrarySidebar({
  folders,
  tags,
  playlists,
  limits,
  activeFilter,
  onFilterChange,
  creatingFolder,
  onSetCreatingFolder,
  newFolderName,
  onSetNewFolderName,
  onCreateFolder,
  creatingTag,
  onSetCreatingTag,
  newTagName,
  onSetNewTagName,
  newTagColor,
  onSetNewTagColor,
  onCreateTag,
  editingSidebarId,
  editingSidebarName,
  onSetEditingSidebarId,
  onSetEditingSidebarName,
  onRenameSidebarItem,
  onDeleteSidebarItem,
  sidebarMenuId,
  onSetSidebarMenuId,
}: LibrarySidebarProps) {
  return (
    <nav className="library-sidebar">
      <button
        className={`sidebar-item${activeFilter === "all" ? " sidebar-item--active" : ""}`}
        onClick={() => onFilterChange("all")}
      >
        All Videos
      </button>
      <button
        className={`sidebar-item${activeFilter === "unfiled" ? " sidebar-item--active" : ""}`}
        onClick={() => onFilterChange("unfiled")}
      >
        Unfiled
      </button>

      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span>Folders</span>
          <button className="sidebar-add-btn" onClick={() => onSetCreatingFolder(true)} title="New folder">+</button>
        </div>
        {creatingFolder && (
          <div style={{ padding: "4px 8px" }}>
            <input
              autoFocus
              value={newFolderName}
              onChange={(e) => onSetNewFolderName(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") onCreateFolder(); if (e.key === "Escape") onSetCreatingFolder(false); }}
              placeholder="Folder name"
              maxLength={limits?.fieldLimits?.folderName ?? 100}
              style={{ width: "100%", padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
            />
          </div>
        )}
        {folders.map((folder) => (
          <div key={folder.id} className="sidebar-item-wrapper" onMouseLeave={() => { if (sidebarMenuId === `folder-${folder.id}`) onSetSidebarMenuId(null); }}>
            {editingSidebarId === `folder-${folder.id}` ? (
              <input
                autoFocus
                value={editingSidebarName}
                onChange={(e) => onSetEditingSidebarName(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") onRenameSidebarItem("folder", folder.id); if (e.key === "Escape") onSetEditingSidebarId(null); }}
                onBlur={() => onRenameSidebarItem("folder", folder.id)}
                style={{ width: "100%", padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
              />
            ) : (
              <button
                className={`sidebar-item${activeFilter === `folder:${folder.id}` ? " sidebar-item--active" : ""}`}
                onClick={() => onFilterChange(`folder:${folder.id}`)}
              >
                <span className="sidebar-item-name">{folder.name}</span>
                <span className="sidebar-item-count">{folder.videoCount}</span>
              </button>
            )}
            <button
              className="sidebar-item-menu-btn"
              onClick={(e) => { e.stopPropagation(); onSetSidebarMenuId(sidebarMenuId === `folder-${folder.id}` ? null : `folder-${folder.id}`); }}
            >
              &#x22EE;
            </button>
            {sidebarMenuId === `folder-${folder.id}` && (
              <div className="sidebar-item-menu" onClick={(e) => e.stopPropagation()}>
                <button onClick={() => { onSetEditingSidebarId(`folder-${folder.id}`); onSetEditingSidebarName(folder.name); onSetSidebarMenuId(null); }}>Rename</button>
                <button onClick={() => { onDeleteSidebarItem("folder", folder.id); onSetSidebarMenuId(null); }}>Delete</button>
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span>Tags</span>
          <button className="sidebar-add-btn" onClick={() => onSetCreatingTag(true)} title="New tag">+</button>
        </div>
        {creatingTag && (
          <div style={{ padding: "4px 8px" }}>
            <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
              <input
                type="color"
                value={newTagColor}
                onChange={(e) => onSetNewTagColor(e.target.value)}
                style={{ width: 24, height: 24, padding: 0, border: "none", cursor: "pointer" }}
              />
              <input
                autoFocus
                value={newTagName}
                onChange={(e) => onSetNewTagName(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") onCreateTag(); if (e.key === "Escape") onSetCreatingTag(false); }}
                placeholder="Tag name"
                maxLength={limits?.fieldLimits?.tagName ?? 50}
                style={{ flex: 1, padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
              />
            </div>
          </div>
        )}
        {tags.map((tag) => (
          <div key={tag.id} className="sidebar-item-wrapper" onMouseLeave={() => { if (sidebarMenuId === `tag-${tag.id}`) onSetSidebarMenuId(null); }}>
            {editingSidebarId === `tag-${tag.id}` ? (
              <input
                autoFocus
                value={editingSidebarName}
                onChange={(e) => onSetEditingSidebarName(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") onRenameSidebarItem("tag", tag.id); if (e.key === "Escape") onSetEditingSidebarId(null); }}
                onBlur={() => onRenameSidebarItem("tag", tag.id)}
                style={{ width: "100%", padding: "4px 8px", fontSize: 13, background: "var(--color-background)", border: "1px solid var(--color-border)", borderRadius: 4, color: "var(--color-text)" }}
              />
            ) : (
              <button
                className={`sidebar-item${activeFilter === `tag:${tag.id}` ? " sidebar-item--active" : ""}`}
                onClick={() => onFilterChange(`tag:${tag.id}`)}
              >
                <span className="tag-dot" style={{ background: tag.color ?? "var(--color-text-secondary)" }} />
                <span className="sidebar-item-name">{tag.name}</span>
                <span className="sidebar-item-count">{tag.videoCount}</span>
              </button>
            )}
            <button
              className="sidebar-item-menu-btn"
              onClick={(e) => { e.stopPropagation(); onSetSidebarMenuId(sidebarMenuId === `tag-${tag.id}` ? null : `tag-${tag.id}`); }}
            >
              &#x22EE;
            </button>
            {sidebarMenuId === `tag-${tag.id}` && (
              <div className="sidebar-item-menu" onClick={(e) => e.stopPropagation()}>
                <button onClick={() => { onSetEditingSidebarId(`tag-${tag.id}`); onSetEditingSidebarName(tag.name); onSetSidebarMenuId(null); }}>Rename</button>
                <button onClick={() => { onDeleteSidebarItem("tag", tag.id); onSetSidebarMenuId(null); }}>Delete</button>
              </div>
            )}
          </div>
        ))}
      </div>

      {playlists.length > 0 && (
        <div className="sidebar-section">
          <div className="sidebar-section-header">
            <span>Playlists</span>
          </div>
          {playlists.map((playlist) => (
            <Link
              key={playlist.id}
              to={`/playlists/${playlist.id}`}
              className="sidebar-item"
              style={{ textDecoration: "none" }}
            >
              <span className="sidebar-item-name">{playlist.title}</span>
              <span className="sidebar-item-count">{playlist.videoCount}</span>
            </Link>
          ))}
        </div>
      )}
    </nav>
  );
}
