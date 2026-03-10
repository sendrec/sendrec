export interface VideoTag {
  id: string;
  name: string;
  color: string | null;
}

export interface Video {
  id: string;
  title: string;
  status: string;
  duration: number;
  shareToken: string;
  shareUrl: string;
  createdAt: string;
  shareExpiresAt: string | null;
  viewCount: number;
  uniqueViewCount: number;
  thumbnailUrl?: string;
  hasPassword: boolean;
  commentMode: string;
  commentCount: number;
  transcriptStatus: string;
  viewNotification: string | null;
  downloadEnabled: boolean;
  emailGateEnabled: boolean;
  ctaText: string | null;
  ctaUrl: string | null;
  suggestedTitle: string | null;
  summaryStatus: string;
  document?: string;
  documentStatus?: string;
  folderId: string | null;
  transcriptionLanguage?: string | null;
  noiseReduction?: boolean;
  pinned: boolean;
  tags: VideoTag[];
  playlists?: { id: string; title: string }[];
}

export interface Folder {
  id: string;
  name: string;
  position: number;
  videoCount: number;
  createdAt: string;
}

export interface Tag {
  id: string;
  name: string;
  color: string | null;
  videoCount: number;
  createdAt: string;
}
