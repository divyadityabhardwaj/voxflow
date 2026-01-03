export namespace history {
	
	export class Transcript {
	    id: number;
	    // Go type: time
	    timestamp: any;
	    app_name: string;
	    raw_text: string;
	    polished_text: string;
	    mode: string;
	
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

