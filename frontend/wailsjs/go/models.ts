export namespace core {
	
	export class LauncherConfig {
	    linuxPatchComplete: boolean;
	    mangoHud: boolean;
	    fsr: boolean;
	    shaderCache: boolean;
	    hdr: boolean;
	    steamFix: boolean;
	    fpsLimit: string;
	    cdn: boolean;
	    wineDllOverrides: string;
	    grafikMod: string;
	    fsrLvl: string;
	
	    static createFrom(source: any = {}) {
	        return new LauncherConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linuxPatchComplete = source["linuxPatchComplete"];
	        this.mangoHud = source["mangoHud"];
	        this.fsr = source["fsr"];
	        this.shaderCache = source["shaderCache"];
	        this.hdr = source["hdr"];
	        this.steamFix = source["steamFix"];
	        this.fpsLimit = source["fpsLimit"];
	        this.cdn = source["cdn"];
	        this.wineDllOverrides = source["wineDllOverrides"];
	        this.grafikMod = source["grafikMod"];
	        this.fsrLvl = source["fsrLvl"];
	    }
	}

}

