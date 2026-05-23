export namespace history {
	
	export class Transcript {
	    id: number;
	    // Go type: time
	    timestamp: any;
	    app_name: string;
	    raw_text: string;
	    polished_text: string;
	    mode: string;
	    llm_provider: string;
	    llm_model: string;
	    translation_time_ms: number;
	    tokens_per_second: number;
	    words_per_second: number;
	
	    static createFrom(source: any = {}) {
	        return new Transcript(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.app_name = source["app_name"];
	        this.raw_text = source["raw_text"];
	        this.polished_text = source["polished_text"];
	        this.mode = source["mode"];
	        this.llm_provider = source["llm_provider"];
	        this.llm_model = source["llm_model"];
	        this.translation_time_ms = source["translation_time_ms"];
	        this.tokens_per_second = source["tokens_per_second"];
	        this.words_per_second = source["words_per_second"];
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

export namespace main {
	
	export class AppRuleDTO {
	    bundle_id: string;
	    app_name?: string;
	    refinement_mode?: string;
	    inject_method?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppRuleDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundle_id = source["bundle_id"];
	        this.app_name = source["app_name"];
	        this.refinement_mode = source["refinement_mode"];
	        this.inject_method = source["inject_method"];
	    }
	}
	export class CheckResult {
	    latency: number;
	    tps: number;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latency = source["latency"];
	        this.tps = source["tps"];
	    }
	}
	export class ConfigResponse {
	    hotkey: string;
	    hands_free_hotkey: string;
	    push_to_talk_hotkey: string;
	    whisper_model: string;
	    whisper_language: string;
	    whisper_threads: number;
	    gemini_model: string;
	    api_key_set: boolean;
	    llm_provider: string;
	    openrouter_model: string;
	    openrouter_api_key_set: boolean;
	    groq_model: string;
	    groq_api_key_set: boolean;
	    cerebras_model: string;
	    cerebras_api_key_set: boolean;
	    local_model: string;
	    local_url: string;
	    refinement_mode: string;
	    mute_system_audio: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hotkey = source["hotkey"];
	        this.hands_free_hotkey = source["hands_free_hotkey"];
	        this.push_to_talk_hotkey = source["push_to_talk_hotkey"];
	        this.whisper_model = source["whisper_model"];
	        this.whisper_language = source["whisper_language"];
	        this.whisper_threads = source["whisper_threads"];
	        this.gemini_model = source["gemini_model"];
	        this.api_key_set = source["api_key_set"];
	        this.llm_provider = source["llm_provider"];
	        this.openrouter_model = source["openrouter_model"];
	        this.openrouter_api_key_set = source["openrouter_api_key_set"];
	        this.groq_model = source["groq_model"];
	        this.groq_api_key_set = source["groq_api_key_set"];
	        this.cerebras_model = source["cerebras_model"];
	        this.cerebras_api_key_set = source["cerebras_api_key_set"];
	        this.local_model = source["local_model"];
	        this.local_url = source["local_url"];
	        this.refinement_mode = source["refinement_mode"];
	        this.mute_system_audio = source["mute_system_audio"];
	    }
	}
	export class FrontmostAppInfo {
	    bundle_id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new FrontmostAppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bundle_id = source["bundle_id"];
	        this.name = source["name"];
	    }
	}
	export class HistoryPage {
	    transcripts: history.Transcript[];
	    next_cursor_ts: string;
	    next_cursor_id: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.transcripts = this.convertValues(source["transcripts"], history.Transcript);
	        this.next_cursor_ts = source["next_cursor_ts"];
	        this.next_cursor_id = source["next_cursor_id"];
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

export namespace whisper {
	
	export class ModelInfo {
	    name: string;
	    description: string;
	    size: number;
	    downloaded: boolean;
	    file_path: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.size = source["size"];
	        this.downloaded = source["downloaded"];
	        this.file_path = source["file_path"];
	    }
	}

}

