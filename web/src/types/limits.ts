export interface FieldLimits {
  title: number;
  playlistTitle: number;
  playlistDescription: number;
  folderName: number;
  tagName: number;
  commentBody: number;
  companyName: number;
  footerText: number;
  customCSS: number;
  webhookURL: number;
  apiKeyName: number;
}

export interface LimitsResponse {
  maxVideosPerMonth: number;
  maxVideoDurationSeconds: number;
  videosUsedThisMonth: number;
  brandingEnabled: boolean;
  aiEnabled: boolean;
  transcriptionEnabled: boolean;
  noiseReductionEnabled: boolean;
  maxPlaylists: number;
  playlistsUsed: number;
  fieldLimits: FieldLimits;
}
