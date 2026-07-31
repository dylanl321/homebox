import { BaseAPI, route } from "../base";

export interface ZebraPrinterSettings {
  printerIp: string;
  printerPort: number;
  labelSize: "1x1" | "2x1" | "2.25x1.25" | "3x2" | "4x2" | "4x6";
  orientation: "portrait" | "landscape";
  darkness: number;
  printSpeed: number;
  printFontSize: number;
}

export class ZebraPrinterApi extends BaseAPI {
  public settings() {
    return this.http.get<ZebraPrinterSettings>({ url: route("/labelmaker/settings") });
  }

  public update(settings: ZebraPrinterSettings) {
    return this.http.put<ZebraPrinterSettings, ZebraPrinterSettings>({
      url: route("/labelmaker/settings"),
      body: settings,
    });
  }

  public test(settings: ZebraPrinterSettings) {
    return this.http.post<ZebraPrinterSettings, { printed: boolean }>({
      url: route("/labelmaker/test"),
      body: settings,
    });
  }
}
