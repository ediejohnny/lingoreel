export namespace app {
	
	export class Capabilities {
	    pause: boolean;
	    seek: boolean;
	    speed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pause = source["pause"];
	        this.seek = source["seek"];
	        this.speed = source["speed"];
	    }
	}
	export class LineNames {
	    primary: string;
	    translation: string;
	
	    static createFrom(source: any = {}) {
	        return new LineNames(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.primary = source["primary"];
	        this.translation = source["translation"];
	    }
	}
	export class Playback {
	    active: boolean;
	    status: string;
	    position: number;
	    duration: number;
	    speed: number;
	    positionText: string;
	    durationText: string;
	    primaryOffset: string;
	    translationOffset: string;
	    translationVisible: boolean;
	    capabilities: Capabilities;
	    lines: LineNames;
	
	    static createFrom(source: any = {}) {
	        return new Playback(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.status = source["status"];
	        this.position = source["position"];
	        this.duration = source["duration"];
	        this.speed = source["speed"];
	        this.positionText = source["positionText"];
	        this.durationText = source["durationText"];
	        this.primaryOffset = source["primaryOffset"];
	        this.translationOffset = source["translationOffset"];
	        this.translationVisible = source["translationVisible"];
	        this.capabilities = this.convertValues(source["capabilities"], Capabilities);
	        this.lines = this.convertValues(source["lines"], LineNames);
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
	export class PlayerInstall {
	    id: string;
	    name: string;
	    benefit: string;
	    autoInstall: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerInstall(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.benefit = source["benefit"];
	        this.autoInstall = source["autoInstall"];
	    }
	}
	export class PlayerOption {
	    id: string;
	    name: string;
	    path: string;
	    supported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.supported = source["supported"];
	    }
	}
	export class Players {
	    choice: string;
	    preferred: string;
	    preferredMissing: boolean;
	    installed: PlayerOption[];
	    recommendations: PlayerInstall[];
	
	    static createFrom(source: any = {}) {
	        return new Players(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.choice = source["choice"];
	        this.preferred = source["preferred"];
	        this.preferredMissing = source["preferredMissing"];
	        this.installed = this.convertValues(source["installed"], PlayerOption);
	        this.recommendations = this.convertValues(source["recommendations"], PlayerInstall);
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
	export class Preferences {
	    theme: string;
	    speeds: number[];
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.speeds = source["speeds"];
	    }
	}
	export class Selection {
	    audio: number;
	    primary: number;
	    translation: number;
	
	    static createFrom(source: any = {}) {
	        return new Selection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.audio = source["audio"];
	        this.primary = source["primary"];
	        this.translation = source["translation"];
	    }
	}
	export class Track {
	    number: number;
	    label: string;
	    language: string;
	    readable: boolean;
	    note: string;
	    warning: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.label = source["label"];
	        this.language = source["language"];
	        this.readable = source["readable"];
	        this.note = source["note"];
	        this.warning = source["warning"];
	    }
	}
	export class Video {
	    path: string;
	    name: string;
	    duration: number;
	    audio: Track[];
	    subtitles: Track[];
	    suggested: Selection;
	    players: Players;
	
	    static createFrom(source: any = {}) {
	        return new Video(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.duration = source["duration"];
	        this.audio = this.convertValues(source["audio"], Track);
	        this.subtitles = this.convertValues(source["subtitles"], Track);
	        this.suggested = this.convertValues(source["suggested"], Selection);
	        this.players = this.convertValues(source["players"], Players);
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
	export class WatchRequest {
	    audio: number;
	    primary: number;
	    translation: number;
	    player: string;
	    alwaysUse: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.audio = source["audio"];
	        this.primary = source["primary"];
	        this.translation = source["translation"];
	        this.player = source["player"];
	        this.alwaysUse = source["alwaysUse"];
	    }
	}

}

