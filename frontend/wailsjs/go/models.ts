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
	export class Karaoke {
	    enabled: boolean;
	    indicator: string;
	    repeat: number;
	
	    static createFrom(source: any = {}) {
	        return new Karaoke(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.indicator = source["indicator"];
	        this.repeat = source["repeat"];
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
	export class Passage {
	    active: boolean;
	    start: string;
	    end: string;
	    lap: number;
	    goal: number;
	    suspended: boolean;
	    atEnd: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Passage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.lap = source["lap"];
	        this.goal = source["goal"];
	        this.suspended = source["suspended"];
	        this.atEnd = source["atEnd"];
	    }
	}
	export class PassageRequest {
	    start: string;
	    end: string;
	    snap: boolean;
	    loop: boolean;
	    gap: number;
	    progressive: boolean;
	    ladder: boolean;
	    goal: number;
	
	    static createFrom(source: any = {}) {
	        return new PassageRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.snap = source["snap"];
	        this.loop = source["loop"];
	        this.gap = source["gap"];
	        this.progressive = source["progressive"];
	        this.ladder = source["ladder"];
	        this.goal = source["goal"];
	    }
	}
	export class PassageTimes {
	    start: string;
	    end: string;
	
	    static createFrom(source: any = {}) {
	        return new PassageTimes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
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
	    karaoke: Karaoke;
	    passage: Passage;
	
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
	        this.karaoke = this.convertValues(source["karaoke"], Karaoke);
	        this.passage = this.convertValues(source["passage"], Passage);
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

