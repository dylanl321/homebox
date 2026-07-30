import { QRLoginApi } from "~~/lib/api/classes/qr-login";
import { Requests } from "~~/lib/requests";
import { route } from "~~/lib/api/base";

/**
 * Fork-local QR login helpers.
 * Kept separate from useAuthContext so upstream auth changes merge cleanly.
 */
export function useQRLoginApi(authenticated = false) {
  const prefs = useViewPreferences();
  const headers: Record<string, string> = {};
  if (authenticated && prefs?.value?.collectionId) {
    headers["X-Tenant"] = prefs.value.collectionId;
  }
  return new QRLoginApi(new Requests("", "", headers));
}

export function useQRLogin() {
  function qrImageUrl(token: string): string {
    if (typeof window === "undefined") return "";
    const loginUrl = `${window.location.origin}/login/qr?token=${encodeURIComponent(token)}`;
    return route(`/qrcode`, { data: encodeURIComponent(loginUrl) });
  }

  /**
   * Exchange a QR token for a session. Server Set-Cookie headers establish the
   * session; callers should hard-navigate afterward so AuthContext reloads.
   */
  async function exchange(token: string, stayLoggedIn = true) {
    return useQRLoginApi(false).exchange(token, stayLoggedIn);
  }

  return { exchange, qrImageUrl };
}
