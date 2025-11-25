export namespace models {
	
	export class Chunk {
	    id: string;
	    projectId: string;
	    filePath: string;
	    content: string;
	    embedding: number[];
	    embeddingModelId?: string;
	    similarity?: number;
	    lineStart: number;
	    lineEnd: number;
	    charStart: number;
	    charEnd: number;
	    createdAt: number;
	    updatedAt: number;
	    language?: string;
	    symbolName?: string;
	    symbolKind?: string;
	    parent?: string;
	    signature?: string;
	    visibility?: string;
	    packageName?: string;
	    docString?: string;
	    tokenCount?: number;
	    isCollapsed?: boolean;
	    sourceCode?: string;
	
	    static createFrom(source: any = {}) {
	        return new Chunk(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.filePath = source["filePath"];
	        this.content = source["content"];
	        this.embedding = source["embedding"];
	        this.embeddingModelId = source["embeddingModelId"];
	        this.similarity = source["similarity"];
	        this.lineStart = source["lineStart"];
	        this.lineEnd = source["lineEnd"];
	        this.charStart = source["charStart"];
	        this.charEnd = source["charEnd"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.language = source["language"];
	        this.symbolName = source["symbolName"];
	        this.symbolKind = source["symbolKind"];
	        this.parent = source["parent"];
	        this.signature = source["signature"];
	        this.visibility = source["visibility"];
	        this.packageName = source["packageName"];
	        this.docString = source["docString"];
	        this.tokenCount = source["tokenCount"];
	        this.isCollapsed = source["isCollapsed"];
	        this.sourceCode = source["sourceCode"];
	    }
	}
	export class EmbeddingCapabilities {
	    onnxRuntimeAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingCapabilities(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.onnxRuntimeAvailable = source["onnxRuntimeAvailable"];
	    }
	}
	export class EmbeddingModelInfo {
	    id: string;
	    displayName: string;
	    backend: string;
	    description?: string;
	    dimension: number;
	    diskSizeBytes?: number;
	    ramRequirementBytes?: number;
	    cpuLatencyMs?: number;
	    isMultilingual: boolean;
	    codeQuality?: string;
	    notes?: string;
	    sourceType: string;
	    sourceUri?: string;
	    localPath?: string;
	    tokenizerUri?: string;
	    tokenizerLocalPath?: string;
	    license?: string;
	    downloadStatus?: string;
	    requiresConversion?: boolean;
	    preferredFilename?: string;
	    createdAt?: number;
	    updatedAt?: number;
	    codeFocus?: string;
	    estimatedTokensPerSecond?: number;
	    supportsQuantization?: boolean;
	    maxSequenceLength?: number;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.backend = source["backend"];
	        this.description = source["description"];
	        this.dimension = source["dimension"];
	        this.diskSizeBytes = source["diskSizeBytes"];
	        this.ramRequirementBytes = source["ramRequirementBytes"];
	        this.cpuLatencyMs = source["cpuLatencyMs"];
	        this.isMultilingual = source["isMultilingual"];
	        this.codeQuality = source["codeQuality"];
	        this.notes = source["notes"];
	        this.sourceType = source["sourceType"];
	        this.sourceUri = source["sourceUri"];
	        this.localPath = source["localPath"];
	        this.tokenizerUri = source["tokenizerUri"];
	        this.tokenizerLocalPath = source["tokenizerLocalPath"];
	        this.license = source["license"];
	        this.downloadStatus = source["downloadStatus"];
	        this.requiresConversion = source["requiresConversion"];
	        this.preferredFilename = source["preferredFilename"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.codeFocus = source["codeFocus"];
	        this.estimatedTokensPerSecond = source["estimatedTokensPerSecond"];
	        this.supportsQuantization = source["supportsQuantization"];
	        this.maxSequenceLength = source["maxSequenceLength"];
	    }
	}
	export class FilePreview {
	    absolutePath: string;
	    relativePath: string;
	    extension: string;
	    size: string;
	    hidden: boolean;
	    lastModified: number;
	
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
	        this.lastModified = source["lastModified"];
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
	export class ONNXRuntimeSettings {
	    sharedLibraryPath: string;
	    activePath?: string;
	    runtimeAvailable: boolean;
	    requiresRestart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ONNXRuntimeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sharedLibraryPath = source["sharedLibraryPath"];
	        this.activePath = source["activePath"];
	        this.runtimeAvailable = source["runtimeAvailable"];
	        this.requiresRestart = source["requiresRestart"];
	    }
	}
	export class ONNXRuntimeTestResult {
	    success: boolean;
	    message: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ONNXRuntimeTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}
	export class OutlineNode {
	    id: string;
	    name: string;
	    kind: string;
	    filePath: string;
	    startLine: number;
	    endLine: number;
	    children?: OutlineNode[];
	
	    static createFrom(source: any = {}) {
	        return new OutlineNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.filePath = source["filePath"];
	        this.startLine = source["startLine"];
	        this.endLine = source["endLine"];
	        this.children = this.convertValues(source["children"], OutlineNode);
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
	export class ProjectEmbeddingModelUsage {
	    modelId: string;
	    chunkCount: number;
	    modelInfo?: EmbeddingModelInfo;
	
	    static createFrom(source: any = {}) {
	        return new ProjectEmbeddingModelUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modelId = source["modelId"];
	        this.chunkCount = source["chunkCount"];
	        this.modelInfo = this.convertValues(source["modelInfo"], EmbeddingModelInfo);
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
	export class ProjectStats {
	    totalFiles: number;
	    totalChunks: number;
	    totalSymbols: number;
	    databaseSize: number;
	    // Go type: time
	    lastIndexedAt?: any;
	    lastIndexedAtUnix?: number;
	    embeddingModels?: ProjectEmbeddingModelUsage[];
	    lastEmbeddingModel?: EmbeddingModelInfo;
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
	        this.lastIndexedAtUnix = source["lastIndexedAtUnix"];
	        this.embeddingModels = this.convertValues(source["embeddingModels"], ProjectEmbeddingModelUsage);
	        this.lastEmbeddingModel = this.convertValues(source["lastEmbeddingModel"], EmbeddingModelInfo);
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
	    embeddingBackend?: string;
	    embeddingModelInfo?: EmbeddingModelInfo;
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
	        this.embeddingBackend = source["embeddingBackend"];
	        this.embeddingModelInfo = this.convertValues(source["embeddingModelInfo"], EmbeddingModelInfo);
	        this.maxResponseBytes = source["maxResponseBytes"];
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
	
	
	
	export class SearchResponse {
	    chunks: Chunk[];
	    totalResults: number;
	    queryTime: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunks = this.convertValues(source["chunks"], Chunk);
	        this.totalResults = source["totalResults"];
	        this.queryTime = source["queryTime"];
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

