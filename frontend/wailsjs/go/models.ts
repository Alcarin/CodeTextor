export namespace models {
	
	export class FilePreview {
	    absolutePath: string;
	    relativePath: string;
	    extension: string;
	    size: string;
	    hidden: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FilePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.absolutePath = source["absolutePath"];
	        this.relativePath = source["relativePath"];
	        this.extension = source["extension"];
	        this.size = source["size"];
	        this.hidden = source["hidden"];
	    }
	}
	export class IndexingProgress {
	    totalFiles: number;
	    processedFiles: number;
	    currentFile: string;
	    status: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new IndexingProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFiles = source["totalFiles"];
	        this.processedFiles = source["processedFiles"];
	        this.currentFile = source["currentFile"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}
	export class ProjectStats {
	    totalFiles: number;
	    totalChunks: number;
	    totalSymbols: number;
	    databaseSize: number;
	    // Go type: time
	    lastIndexedAt?: any;
	    isIndexing: boolean;
	    indexingProgress: number;
	
	    static createFrom(source: any = {}) {
	        return new ProjectStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFiles = source["totalFiles"];
	        this.totalChunks = source["totalChunks"];
	        this.totalSymbols = source["totalSymbols"];
	        this.databaseSize = source["databaseSize"];
	        this.lastIndexedAt = this.convertValues(source["lastIndexedAt"], null);
	        this.isIndexing = source["isIndexing"];
	        this.indexingProgress = source["indexingProgress"];
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
	export class ProjectConfig {
	    includePaths: string[];
	    excludePatterns: string[];
	    fileExtensions: string[];
	    rootPath: string;
	    autoExcludeHidden: boolean;
	    continuousIndexing: boolean;
	    chunkSizeMin: number;
	    chunkSizeMax: number;
	    embeddingModel: string;
	    maxResponseBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ProjectConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.includePaths = source["includePaths"];
	        this.excludePatterns = source["excludePatterns"];
	        this.fileExtensions = source["fileExtensions"];
	        this.rootPath = source["rootPath"];
	        this.autoExcludeHidden = source["autoExcludeHidden"];
	        this.continuousIndexing = source["continuousIndexing"];
	        this.chunkSizeMin = source["chunkSizeMin"];
	        this.chunkSizeMax = source["chunkSizeMax"];
	        this.embeddingModel = source["embeddingModel"];
	        this.maxResponseBytes = source["maxResponseBytes"];
	    }
	}
	export class Project {
	    id: string;
	    name: string;
	    description: string;
	    createdAt: number;
	    updatedAt: number;
	    config: ProjectConfig;
	    isIndexing: boolean;
	    stats?: ProjectStats;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.config = this.convertValues(source["config"], ProjectConfig);
	        this.isIndexing = source["isIndexing"];
	        this.stats = this.convertValues(source["stats"], ProjectStats);
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

