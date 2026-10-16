export function draftKey(userID?: string): string {
  return userID ? `nsm:report-draft:user:${userID}` : "nsm:report-draft:guest";
}

// Keep a guest's draft through registration, without showing another
// authenticated account's draft in the same tab.
export function carryGuestDraft(userID: string) {
  try {
    const guest = sessionStorage.getItem(draftKey());
    if (guest && !sessionStorage.getItem(draftKey(userID))) {
      sessionStorage.setItem(draftKey(userID), guest);
    }
    sessionStorage.removeItem(draftKey());
  } catch {
    /* Storage may be unavailable; authentication still works. */
  }
}

export function clearPrivateStorage(includeGuest = false) {
  try {
    const keys = Array.from({ length: sessionStorage.length }, (_, i) =>
      sessionStorage.key(i),
    );
    for (const key of keys) {
      if (
        key &&
        (key.startsWith("nsm:challenge:") ||
          key.startsWith("nsm:report-draft:user:") ||
          key === "nsm:report-draft" ||
          (includeGuest && key === draftKey()))
      ) {
        sessionStorage.removeItem(key);
      }
    }
  } catch {
    /* No storage means there is no persisted client state to clear. */
  }
}
