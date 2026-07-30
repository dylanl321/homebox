import { BaseAPI, route } from "../base";
import type { TokenResponse } from "../types/data-contracts";
import type { QRLoginCreateResponse, QRLoginExchangeRequest } from "../types/qr-login";

/**
 * Fork-local QR login API client.
 * Kept out of UserApi/PublicApi so upstream client regenerations merge cleanly.
 */
export class QRLoginApi extends BaseAPI {
  public create() {
    return this.http.post<object, QRLoginCreateResponse>({
      url: route("/users/self/qr-login"),
      body: {},
    });
  }

  public exchange(token: string, stayLoggedIn = true) {
    return this.http.post<QRLoginExchangeRequest, TokenResponse>({
      url: route("/users/login/qr"),
      body: { token, stayLoggedIn },
    });
  }
}
