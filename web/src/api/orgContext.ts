const STORAGE_KEY = "sendrec-org-id";

let currentOrgId: string | null = null;
let listeners: Array<() => void> = [];

if (typeof window !== "undefined") {
  currentOrgId = localStorage.getItem(STORAGE_KEY);
}

export function getCurrentOrgId(): string | null {
  return currentOrgId;
}

export function setCurrentOrgId(orgId: string | null): void {
  currentOrgId = orgId;
  if (orgId) {
    localStorage.setItem(STORAGE_KEY, orgId);
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
  listeners.forEach((l) => l());
}

export function subscribeToOrgChanges(listener: () => void): () => void {
  listeners.push(listener);
  return () => {
    listeners = listeners.filter((l) => l !== listener);
  };
}

// A workspace's details changed (name, icon…). Each useOrganization instance
// holds its own copy of the list — the layout's switcher among them — so they
// all refetch rather than show the old values until a reload.
let updateListeners: Array<() => void> = [];

export function announceOrgUpdate(): void {
  updateListeners.forEach((l) => l());
}

export function subscribeToOrgUpdates(listener: () => void): () => void {
  updateListeners.push(listener);
  return () => {
    updateListeners = updateListeners.filter((l) => l !== listener);
  };
}
