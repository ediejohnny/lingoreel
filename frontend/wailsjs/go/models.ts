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
	export class GeneralSettings {
	    theme: string;
	    syncNotice: boolean;
	    activeProfile: string;
	    recommendationsHidden: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GeneralSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.syncNotice = source["syncNotice"];
	        this.activeProfile = source["activeProfile"];
	        this.recommendationsHidden = source["recommendationsHidden"];
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
	export class KaraokeSettings {
	    enabled: boolean;
	    minGap: number;
	    lead: number;
	    indicator: string;
	    previewColor: string;
	    fill: boolean;
	    fillColor: string;
	    repeat: number;
	
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
	
	    static createFrom(source: any = {}) {
	        return new Preferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.speeds = source["speeds"];
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
	    }
	}
	export class PreviewRequest {
	    primary: StyleSettings;
	    translation: StyleSettings;
	    layout: LayoutSettings;
	    width: number;
	    height: number;
	    scale: number;
	
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
	    speed: number;
	    syncStep: number;
	    syncFastStep: number;
	    karaoke: KaraokeSettings;
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
	        this.speed = source["speed"];
	        this.syncStep = source["syncStep"];
	        this.syncFastStep = source["syncFastStep"];
	        this.karaoke = this.convertValues(source["karaoke"], KaraokeSettings);
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
	    }
	}
	export class SettingsOptions {
	    fonts: string[];
	    presets: Option[];
	    languages: Option[];
	    speeds: number[];
	    limits: SettingsLimits;
	    defaults: ProfileSettings;
	
	    static createFrom(source: any = {}) {
	        return new SettingsOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fonts = source["fonts"];
	        this.presets = this.convertValues(source["presets"], Option);
	        this.languages = this.convertValues(source["languages"], Option);
	        this.speeds = source["speeds"];
	        this.limits = this.convertValues(source["limits"], SettingsLimits);
	        this.defaults = this.convertValues(source["defaults"], ProfileSettings);
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
	export class SettingsView {
	    general: GeneralSettings;
	    profiles: string[];
	    profile: ProfileSettings;
	    players: PlayerSetting[];
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
	        this.players = this.convertValues(source["players"], PlayerSetting);
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

