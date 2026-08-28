export namespace main {
	
	export class GameSettings {
	    mangoHud: boolean;
	    fsr: boolean;
	    shaderCache: boolean;
	    hdr: boolean;
	    steamFix: boolean;
	    cdn: boolean;
	    fpsLimit: string;
	    wineDllOverrides: string;
	    grafikMod: string;
	    fsrLvl: string;
	
	    static createFrom(source: any = {}) {
	        return new GameSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mangoHud = source["mangoHud"];
	        this.fsr = source["fsr"];
	        this.shaderCache = source["shaderCache"];
	        this.hdr = source["hdr"];
	        this.steamFix = source["steamFix"];
	        this.cdn = source["cdn"];
	        this.fpsLimit = source["fpsLimit"];
	        this.wineDllOverrides = source["wineDllOverrides"];
	        this.grafikMod = source["grafikMod"];
	        this.fsrLvl = source["fsrLvl"];
	    }
	}

}

