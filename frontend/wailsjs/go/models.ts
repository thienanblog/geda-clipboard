export namespace main {
	
	export class Environment {
	    platform: string;
	    modifierName: string;
	    version: string;
	    canPaste: boolean;
	    pasteSupported: boolean;
	    notificationStatus: string;
	    hotkeyError: string;
	
	    static createFrom(source: any = {}) {
	        return new Environment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.modifierName = source["modifierName"];
	        this.version = source["version"];
	        this.canPaste = source["canPaste"];
	        this.pasteSupported = source["pasteSupported"];
	        this.notificationStatus = source["notificationStatus"];
	        this.hotkeyError = source["hotkeyError"];
	    }
	}

}

export namespace settings {
	
	export class Settings {
	    maxItems: number;
	    notifyOnCopy: boolean;
	    notifyOnPaste: boolean;
	    pasteOnSelect: boolean;
	    hotkey: string;
	    launchAtLogin: boolean;
	    showDockIcon: boolean;
	    ignoredApps: string[];
	    ignoreConcealed: boolean;
	    ignoreTransient: boolean;
	    captureImages: boolean;
	    popupWidth: number;
	    popupHeight: number;
	    popupPlacement: string;
	    previewOnHover: boolean;
	    layoutVersion: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxItems = source["maxItems"];
	        this.notifyOnCopy = source["notifyOnCopy"];
	        this.notifyOnPaste = source["notifyOnPaste"];
	        this.pasteOnSelect = source["pasteOnSelect"];
	        this.hotkey = source["hotkey"];
	        this.launchAtLogin = source["launchAtLogin"];
	        this.showDockIcon = source["showDockIcon"];
	        this.ignoredApps = source["ignoredApps"];
	        this.ignoreConcealed = source["ignoreConcealed"];
	        this.ignoreTransient = source["ignoreTransient"];
	        this.captureImages = source["captureImages"];
	        this.popupWidth = source["popupWidth"];
	        this.popupHeight = source["popupHeight"];
	        this.popupPlacement = source["popupPlacement"];
	        this.previewOnHover = source["previewOnHover"];
	        this.layoutVersion = source["layoutVersion"];
	    }
	}

}

export namespace store {
	
	export class Item {
	    id: string;
	    kind: string;
	    text?: string;
	    imageFile?: string;
	    thumb?: string;
	    imageW?: number;
	    imageH?: number;
	    bytes?: number;
	    hash: string;
	    sourceApp?: string;
	    sourceIconKey?: string;
	    sourceIcon?: string;
	    // Go type: time
	    firstCopy: any;
	    // Go type: time
	    lastCopy: any;
	    copyCount: number;
	    pinned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.text = source["text"];
	        this.imageFile = source["imageFile"];
	        this.thumb = source["thumb"];
	        this.imageW = source["imageW"];
	        this.imageH = source["imageH"];
	        this.bytes = source["bytes"];
	        this.hash = source["hash"];
	        this.sourceApp = source["sourceApp"];
	        this.sourceIconKey = source["sourceIconKey"];
	        this.sourceIcon = source["sourceIcon"];
	        this.firstCopy = this.convertValues(source["firstCopy"], null);
	        this.lastCopy = this.convertValues(source["lastCopy"], null);
	        this.copyCount = source["copyCount"];
	        this.pinned = source["pinned"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace tray {
	
	export class Rect {
	    X: number;
	    Y: number;
	    W: number;
	    H: number;
	
	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.X = source["X"];
	        this.Y = source["Y"];
	        this.W = source["W"];
	        this.H = source["H"];
	    }
	}
	export class Anchor {
	    Icon: Rect;
	    Work: Rect;
	
	    static createFrom(source: any = {}) {
	        return new Anchor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Icon = this.convertValues(source["Icon"], Rect);
	        this.Work = this.convertValues(source["Work"], Rect);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

