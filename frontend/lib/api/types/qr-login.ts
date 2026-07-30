export interface QRLoginCreateResponse {
  expiresAt: Date | string;
  token: string;
}

export interface QRLoginExchangeRequest {
  stayLoggedIn?: boolean;
  token: string;
}
