export namespace app {
	
	export class EditSettings {
	    background: string;
	    text: string;
	    hint: string;
	    hintSize: number;
	
	    static createFrom(source: any = {}) {
	        return new EditSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.background = source["background"];
	        this.text = source["text"];
	        this.hint = source["hint"];
	        this.hintSize = source["hintSize"];
	    }
	}
	export class BarSettings {
	    panel: string;
	    menu: string;
	    accent: string;
	    icon: string;
	    loud: string;
	    fontFamily: string;
	    size: number;
	    hideDelay: number;
	
	    static createFrom(source: any = {}) {
	        return new BarSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.panel = source["panel"];
	        this.menu = source["menu"];
	        this.accent = source["accent"];
	        this.icon = source["icon"];
	        this.loud = source["loud"];
	        this.fontFamily = source["fontFamily"];
	        this.size = source["size"];
	        this.hideDelay = source["hideDelay"];
	    }
	}
	export class AppearanceSettings {
	    bar: BarSettings;
	    edit: EditSettings;
	
	    static createFrom(source: any = {}) {
	        return new AppearanceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bar = this.convertValues(source["bar"], BarSettings);
	        this.edit = this.convertValues(source["edit"], EditSettings);
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
	export class BarPreviewRequest {
	    bar: BarSettings;
	    width: number;
	    height: number;
	    scale: number;
	
	    static createFrom(source: any = {}) {
	        return new BarPreviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bar = this.convertValues(source["bar"], BarSettings);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.scale = source["scale"];
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
	export class Fact {
	    label: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Fact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.value = source["value"];
	    }
	}
	export class Diagnostics {
	    items: Fact[];
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Fact);
	        this.text = source["text"];
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
	export class EditRequest {
	    text: string;
	    start: string;
	    end: string;
	
	    static createFrom(source: any = {}) {
	        return new EditRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.start = source["start"];
	        this.end = source["end"];
	    }
	}
	
	export class Editing {
	    active: boolean;
	    line: string;
	    lineName: string;
	    original: string;
	    text: string;
	    start: string;
	    end: string;
	    originalStart: string;
	    originalEnd: string;
	    edited: boolean;
	    canUndo: boolean;
	    canRedo: boolean;
	    staleEdits: number;
	
	    static createFrom(source: any = {}) {
	        return new Editing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.line = source["line"];
	        this.lineName = source["lineName"];
	        this.original = source["original"];
	        this.text = source["text"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.originalStart = source["originalStart"];
	        this.originalEnd = source["originalEnd"];
	        this.edited = source["edited"];
	        this.canUndo = source["canUndo"];
	        this.canRedo = source["canRedo"];
	        this.staleEdits = source["staleEdits"];
	    }
	}
	
	export class FillCheck {
	    used: string;
	    changed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FillCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.used = source["used"];
	        this.changed = source["changed"];
	    }
	}
	export class Option {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new Option(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class GeneralSettings {
	    theme: string;
	    syncNotice: boolean;
	    activeProfile: string;
	    alwaysOnTop: boolean;
	    recommendationsHidden: boolean;
	    language: string;
	    languages: Option[];
	
	    static createFrom(source: any = {}) {
	        return new GeneralSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.syncNotice = source["syncNotice"];
	        this.activeProfile = source["activeProfile"];
	        this.alwaysOnTop = source["alwaysOnTop"];
	        this.recommendationsHidden = source["recommendationsHidden"];
	        this.language = source["language"];
	        this.languages = this.convertValues(source["languages"], Option);
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
	export class InterfaceSettings {
	    accent: string;
	    fontFamily: string;
	    textSize: number;
	
	    static createFrom(source: any = {}) {
	        return new InterfaceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accent = source["accent"];
	        this.fontFamily = source["fontFamily"];
	        this.textSize = source["textSize"];
	    }
	}
	export class Karaoke {
	    enabled: boolean;
	    indicator: string;
	    repeat: number;
	    voice: string;
	
	    static createFrom(source: any = {}) {
	        return new Karaoke(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.indicator = source["indicator"];
	        this.repeat = source["repeat"];
	        this.voice = source["voice"];
	    }
	}
	export class KaraokeSettings {
	    enabled: boolean;
	    minGap: number;
	    lead: number;
	    indicator: string;
	    previewColor: string;
	    fill: boolean;
	    fillColor: string;
	    repeat: number;
	    barThickness: number;
	    barColor: string;
	    countdownSize: number;
	
	    static createFrom(source: any = {}) {
	        return new KaraokeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.minGap = source["minGap"];
	        this.lead = source["lead"];
	        this.indicator = source["indicator"];
	        this.previewColor = source["previewColor"];
	        this.fill = source["fill"];
	        this.fillColor = source["fillColor"];
	        this.repeat = source["repeat"];
	        this.barThickness = source["barThickness"];
	        this.barColor = source["barColor"];
	        this.countdownSize = source["countdownSize"];
	    }
	}
	export class LanguageSettings {
	    audio: string;
	    primary: string;
	    translation: string;
	
	    static createFrom(source: any = {}) {
	        return new LanguageSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.audio = source["audio"];
	        this.primary = source["primary"];
	        this.translation = source["translation"];
	    }
	}
	export class LayoutSettings {
	    marginLeft: number;
	    marginRight: number;
	    marginTop: number;
	    marginBottom: number;
	    maxWidth: number;
	    lineGap: number;
	
	    static createFrom(source: any = {}) {
	        return new LayoutSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.marginLeft = source["marginLeft"];
	        this.marginRight = source["marginRight"];
	        this.marginTop = source["marginTop"];
	        this.marginBottom = source["marginBottom"];
	        this.maxWidth = source["maxWidth"];
	        this.lineGap = source["lineGap"];
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
	export class SelectionSettings {
	    color: string;
	    edge: string;
	    width: number;
	    gap: number;
	    radius: number;
	
	    static createFrom(source: any = {}) {
	        return new SelectionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.color = source["color"];
	        this.edge = source["edge"];
	        this.width = source["width"];
	        this.gap = source["gap"];
	        this.radius = source["radius"];
	    }
	}
	export class NoticeSettings {
	    fontFamily: string;
	    fontSize: number;
	    color: string;
	    background: string;
	    side: string;
	    margin: number;
	    maxWidth: number;
	
	    static createFrom(source: any = {}) {
	        return new NoticeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fontFamily = source["fontFamily"];
	        this.fontSize = source["fontSize"];
	        this.color = source["color"];
	        this.background = source["background"];
	        this.side = source["side"];
	        this.margin = source["margin"];
	        this.maxWidth = source["maxWidth"];
	    }
	}
	export class MarkSettings {
	    notice: NoticeSettings;
	    noticeDuration: number;
	    selection: SelectionSettings;
	
	    static createFrom(source: any = {}) {
	        return new MarkSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notice = this.convertValues(source["notice"], NoticeSettings);
	        this.noticeDuration = source["noticeDuration"];
	        this.selection = this.convertValues(source["selection"], SelectionSettings);
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
	
	
	export class PanelState {
	    open: boolean;
	    width: number;
	
	    static createFrom(source: any = {}) {
	        return new PanelState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.open = source["open"];
	        this.width = source["width"];
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
	export class PassagePlan {
	    loop: boolean;
	    gap: number;
	    progressive: boolean;
	    ladder: boolean;
	    goal: number;
	
	    static createFrom(source: any = {}) {
	        return new PassagePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loop = source["loop"];
	        this.gap = source["gap"];
	        this.progressive = source["progressive"];
	        this.ladder = source["ladder"];
	        this.goal = source["goal"];
	    }
	}
	export class PassageOptions {
	    snap: boolean;
	    plan: PassagePlan;
	
	    static createFrom(source: any = {}) {
	        return new PassageOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snap = source["snap"];
	        this.plan = this.convertValues(source["plan"], PassagePlan);
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
	export class PlaybackTracks {
	    audio: Track[];
	    subtitles: Track[];
	    current: Selection;
	    audioSwitch: boolean;
	    loading: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlaybackTracks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.audio = this.convertValues(source["audio"], Track);
	        this.subtitles = this.convertValues(source["subtitles"], Track);
	        this.current = this.convertValues(source["current"], Selection);
	        this.audioSwitch = source["audioSwitch"];
	        this.loading = source["loading"];
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
	export class SavedPassage {
	    name: string;
	    start: string;
	    end: string;
	    plan: PassagePlan;
	    summary: string;
	    outside: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SavedPassage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.plan = this.convertValues(source["plan"], PassagePlan);
	        this.summary = source["summary"];
	        this.outside = source["outside"];
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
	export class Playback {
	    active: boolean;
	    embedded: boolean;
	    status: string;
	    position: number;
	    duration: number;
	    speed: number;
	    positionText: string;
	    durationText: string;
	    primaryOffset: string;
	    translationOffset: string;
	    translationVisible: boolean;
	    mode: string;
	    capabilities: Capabilities;
	    lines: LineNames;
	    karaoke: Karaoke;
	    passage: Passage;
	    editing: Editing;
	    savedPassages: SavedPassage[];
	    markedStart: string;
	    passageOptions: PassageOptions;
	    projectNote: string;
	    health: string;
	    tracks: PlaybackTracks;
	
	    static createFrom(source: any = {}) {
	        return new Playback(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.embedded = source["embedded"];
	        this.status = source["status"];
	        this.position = source["position"];
	        this.duration = source["duration"];
	        this.speed = source["speed"];
	        this.positionText = source["positionText"];
	        this.durationText = source["durationText"];
	        this.primaryOffset = source["primaryOffset"];
	        this.translationOffset = source["translationOffset"];
	        this.translationVisible = source["translationVisible"];
	        this.mode = source["mode"];
	        this.capabilities = this.convertValues(source["capabilities"], Capabilities);
	        this.lines = this.convertValues(source["lines"], LineNames);
	        this.karaoke = this.convertValues(source["karaoke"], Karaoke);
	        this.passage = this.convertValues(source["passage"], Passage);
	        this.editing = this.convertValues(source["editing"], Editing);
	        this.savedPassages = this.convertValues(source["savedPassages"], SavedPassage);
	        this.markedStart = source["markedStart"];
	        this.passageOptions = this.convertValues(source["passageOptions"], PassageOptions);
	        this.projectNote = source["projectNote"];
	        this.health = source["health"];
	        this.tracks = this.convertValues(source["tracks"], PlaybackTracks);
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
	    fixedAudio: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.supported = source["supported"];
	        this.fixedAudio = source["fixedAudio"];
	    }
	}
	export class PlayerSetting {
	    id: string;
	    name: string;
	    supported: boolean;
	    path: string;
	    custom: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.supported = source["supported"];
	        this.path = source["path"];
	        this.custom = source["custom"];
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
	    panel: PanelState;
	    language: string;
	    interface: InterfaceSettings;
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.speeds = source["speeds"];
	        this.panel = this.convertValues(source["panel"], PanelState);
	        this.language = source["language"];
	        this.interface = this.convertValues(source["interface"], InterfaceSettings);
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
	export class StyleSettings {
	    fontFamily: string;
	    fontSize: number;
	    responsive: boolean;
	    bold: boolean;
	    italic: boolean;
	    color: string;
	    opacity: number;
	    outline: boolean;
	    outlineColor: string;
	    outlineWidth: number;
	    shadow: boolean;
	    shadowColor: string;
	    shadowX: number;
	    shadowY: number;
	    shadowBlur: number;
	    background: boolean;
	    backgroundColor: string;
	    backgroundMargin: number;
	    position: number;
	    alignment: string;
	    maxLines: number;
	    lineSpacing: number;
	
	    static createFrom(source: any = {}) {
	        return new StyleSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fontFamily = source["fontFamily"];
	        this.fontSize = source["fontSize"];
	        this.responsive = source["responsive"];
	        this.bold = source["bold"];
	        this.italic = source["italic"];
	        this.color = source["color"];
	        this.opacity = source["opacity"];
	        this.outline = source["outline"];
	        this.outlineColor = source["outlineColor"];
	        this.outlineWidth = source["outlineWidth"];
	        this.shadow = source["shadow"];
	        this.shadowColor = source["shadowColor"];
	        this.shadowX = source["shadowX"];
	        this.shadowY = source["shadowY"];
	        this.shadowBlur = source["shadowBlur"];
	        this.background = source["background"];
	        this.backgroundColor = source["backgroundColor"];
	        this.backgroundMargin = source["backgroundMargin"];
	        this.position = source["position"];
	        this.alignment = source["alignment"];
	        this.maxLines = source["maxLines"];
	        this.lineSpacing = source["lineSpacing"];
	    }
	}
	export class PreviewRequest {
	    primary: StyleSettings;
	    translation: StyleSettings;
	    layout: LayoutSettings;
	    width: number;
	    height: number;
	    scale: number;
	    highlight: string;
	    view: string;
	    karaoke: KaraokeSettings;
	    marks: MarkSettings;
	
	    static createFrom(source: any = {}) {
	        return new PreviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.primary = this.convertValues(source["primary"], StyleSettings);
	        this.translation = this.convertValues(source["translation"], StyleSettings);
	        this.layout = this.convertValues(source["layout"], LayoutSettings);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.scale = source["scale"];
	        this.highlight = source["highlight"];
	        this.view = source["view"];
	        this.karaoke = this.convertValues(source["karaoke"], KaraokeSettings);
	        this.marks = this.convertValues(source["marks"], MarkSettings);
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
	export class ShortcutSetting {
	    command: string;
	    name: string;
	    category: string;
	    chord: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new ShortcutSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.command = source["command"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.chord = source["chord"];
	        this.label = source["label"];
	    }
	}
	export class ProfileSettings {
	    name: string;
	    preferredPlayer: string;
	    languages: LanguageSettings;
	    primary: StyleSettings;
	    translation: StyleSettings;
	    layout: LayoutSettings;
	    marks: MarkSettings;
	    speed: number;
	    syncStep: number;
	    syncFastStep: number;
	    precisionStep: number;
	    karaoke: KaraokeSettings;
	    mode: string;
	    reveal: string;
	    shortcuts: ShortcutSetting[];
	
	    static createFrom(source: any = {}) {
	        return new ProfileSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.preferredPlayer = source["preferredPlayer"];
	        this.languages = this.convertValues(source["languages"], LanguageSettings);
	        this.primary = this.convertValues(source["primary"], StyleSettings);
	        this.translation = this.convertValues(source["translation"], StyleSettings);
	        this.layout = this.convertValues(source["layout"], LayoutSettings);
	        this.marks = this.convertValues(source["marks"], MarkSettings);
	        this.speed = source["speed"];
	        this.syncStep = source["syncStep"];
	        this.syncFastStep = source["syncFastStep"];
	        this.precisionStep = source["precisionStep"];
	        this.karaoke = this.convertValues(source["karaoke"], KaraokeSettings);
	        this.mode = source["mode"];
	        this.reveal = source["reveal"];
	        this.shortcuts = this.convertValues(source["shortcuts"], ShortcutSetting);
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
	export class Resume {
	    offered: boolean;
	    position: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Resume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.offered = source["offered"];
	        this.position = source["position"];
	        this.text = source["text"];
	    }
	}
	
	
	
	export class SettingsLimits {
	    fontSizeMin: number;
	    fontSizeMax: number;
	    outlineMax: number;
	    shadowOffsetMax: number;
	    shadowBlurMax: number;
	    paddingMax: number;
	    linesMin: number;
	    linesMax: number;
	    marginMax: number;
	    gapMin: number;
	    gapMax: number;
	    leadMin: number;
	    leadMax: number;
	    repeatMax: number;
	    syncStepMin: number;
	    syncStepMax: number;
	    syncFastStepMax: number;
	    profileNameMax: number;
	    precisionStepMin: number;
	    precisionStepMax: number;
	    lineSpacingMin: number;
	    lineSpacingMax: number;
	    lineGapMax: number;
	    noticeSizeMax: number;
	    noticeMarginMax: number;
	    noticeWidthMin: number;
	    noticeDurationMin: number;
	    noticeDurationMax: number;
	    selectionWidthMin: number;
	    selectionWidthMax: number;
	    selectionGapMax: number;
	    selectionRadiusMax: number;
	    barThicknessMin: number;
	    barThicknessMax: number;
	    countdownSizeMin: number;
	    countdownSizeMax: number;
	    barSizeMin: number;
	    barSizeMax: number;
	    hideDelayMin: number;
	    hideDelayMax: number;
	    hintSizeMin: number;
	    hintSizeMax: number;
	    textSizeMin: number;
	    textSizeMax: number;
	
	    static createFrom(source: any = {}) {
	        return new SettingsLimits(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fontSizeMin = source["fontSizeMin"];
	        this.fontSizeMax = source["fontSizeMax"];
	        this.outlineMax = source["outlineMax"];
	        this.shadowOffsetMax = source["shadowOffsetMax"];
	        this.shadowBlurMax = source["shadowBlurMax"];
	        this.paddingMax = source["paddingMax"];
	        this.linesMin = source["linesMin"];
	        this.linesMax = source["linesMax"];
	        this.marginMax = source["marginMax"];
	        this.gapMin = source["gapMin"];
	        this.gapMax = source["gapMax"];
	        this.leadMin = source["leadMin"];
	        this.leadMax = source["leadMax"];
	        this.repeatMax = source["repeatMax"];
	        this.syncStepMin = source["syncStepMin"];
	        this.syncStepMax = source["syncStepMax"];
	        this.syncFastStepMax = source["syncFastStepMax"];
	        this.profileNameMax = source["profileNameMax"];
	        this.precisionStepMin = source["precisionStepMin"];
	        this.precisionStepMax = source["precisionStepMax"];
	        this.lineSpacingMin = source["lineSpacingMin"];
	        this.lineSpacingMax = source["lineSpacingMax"];
	        this.lineGapMax = source["lineGapMax"];
	        this.noticeSizeMax = source["noticeSizeMax"];
	        this.noticeMarginMax = source["noticeMarginMax"];
	        this.noticeWidthMin = source["noticeWidthMin"];
	        this.noticeDurationMin = source["noticeDurationMin"];
	        this.noticeDurationMax = source["noticeDurationMax"];
	        this.selectionWidthMin = source["selectionWidthMin"];
	        this.selectionWidthMax = source["selectionWidthMax"];
	        this.selectionGapMax = source["selectionGapMax"];
	        this.selectionRadiusMax = source["selectionRadiusMax"];
	        this.barThicknessMin = source["barThicknessMin"];
	        this.barThicknessMax = source["barThicknessMax"];
	        this.countdownSizeMin = source["countdownSizeMin"];
	        this.countdownSizeMax = source["countdownSizeMax"];
	        this.barSizeMin = source["barSizeMin"];
	        this.barSizeMax = source["barSizeMax"];
	        this.hideDelayMin = source["hideDelayMin"];
	        this.hideDelayMax = source["hideDelayMax"];
	        this.hintSizeMin = source["hintSizeMin"];
	        this.hintSizeMax = source["hintSizeMax"];
	        this.textSizeMin = source["textSizeMin"];
	        this.textSizeMax = source["textSizeMax"];
	    }
	}
	export class SettingsOptions {
	    fonts: string[];
	    presets: Option[];
	    languages: Option[];
	    speeds: number[];
	    modes: Option[];
	    limits: SettingsLimits;
	    defaults: ProfileSettings;
	    appearanceDefaults: AppearanceSettings;
	    editHint: string;
	    interfaceDefaults: InterfaceSettings;
	
	    static createFrom(source: any = {}) {
	        return new SettingsOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fonts = source["fonts"];
	        this.presets = this.convertValues(source["presets"], Option);
	        this.languages = this.convertValues(source["languages"], Option);
	        this.speeds = source["speeds"];
	        this.modes = this.convertValues(source["modes"], Option);
	        this.limits = this.convertValues(source["limits"], SettingsLimits);
	        this.defaults = this.convertValues(source["defaults"], ProfileSettings);
	        this.appearanceDefaults = this.convertValues(source["appearanceDefaults"], AppearanceSettings);
	        this.editHint = source["editHint"];
	        this.interfaceDefaults = this.convertValues(source["interfaceDefaults"], InterfaceSettings);
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
	export class VoiceSettings {
	    follow: boolean;
	    sensitivity: string;
	    delayMs: number;
	    evenWhenBusy: boolean;
	    ffmpegPath: string;
	    ffmpegFound: string;
	    canInstall: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VoiceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.follow = source["follow"];
	        this.sensitivity = source["sensitivity"];
	        this.delayMs = source["delayMs"];
	        this.evenWhenBusy = source["evenWhenBusy"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffmpegFound = source["ffmpegFound"];
	        this.canInstall = source["canInstall"];
	    }
	}
	export class SettingsView {
	    general: GeneralSettings;
	    profiles: string[];
	    profile: ProfileSettings;
	    appearance: AppearanceSettings;
	    players: PlayerSetting[];
	    voice: VoiceSettings;
	    options: SettingsOptions;
	    folder: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.general = this.convertValues(source["general"], GeneralSettings);
	        this.profiles = source["profiles"];
	        this.profile = this.convertValues(source["profile"], ProfileSettings);
	        this.appearance = this.convertValues(source["appearance"], AppearanceSettings);
	        this.players = this.convertValues(source["players"], PlayerSetting);
	        this.voice = this.convertValues(source["voice"], VoiceSettings);
	        this.options = this.convertValues(source["options"], SettingsOptions);
	        this.folder = source["folder"];
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
	export class ShortcutCheck {
	    chord: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new ShortcutCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chord = source["chord"];
	        this.label = source["label"];
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
	    mode: string;
	    modes: Option[];
	    projectNote: string;
	    languageNote: string;
	    savedPassages: SavedPassage[];
	    resume: Resume;
	
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
	        this.mode = source["mode"];
	        this.modes = this.convertValues(source["modes"], Option);
	        this.projectNote = source["projectNote"];
	        this.languageNote = source["languageNote"];
	        this.savedPassages = this.convertValues(source["savedPassages"], SavedPassage);
	        this.resume = this.convertValues(source["resume"], Resume);
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
	export class VideoLayout {
	    visible: boolean;
	    top: number;
	    right: number;
	    bottom: number;
	    left: number;
	
	    static createFrom(source: any = {}) {
	        return new VideoLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.visible = source["visible"];
	        this.top = source["top"];
	        this.right = source["right"];
	        this.bottom = source["bottom"];
	        this.left = source["left"];
	    }
	}
	
	export class WatchRequest {
	    audio: number;
	    primary: number;
	    translation: number;
	    player: string;
	    alwaysUse: boolean;
	    mode: string;
	    resume: boolean;
	    passage: string;
	
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
	        this.mode = source["mode"];
	        this.resume = source["resume"];
	        this.passage = source["passage"];
	    }
	}

}

