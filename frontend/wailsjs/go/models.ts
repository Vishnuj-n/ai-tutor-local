export namespace ui {
	
	export class IngestionStatusRow {
	    notebook_name: string;
	    filename: string;
	    status: string;
	    progress_pct: number;
	
	    static createFrom(source: any = {}) {
	        return new IngestionStatusRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notebook_name = source["notebook_name"];
	        this.filename = source["filename"];
	        this.status = source["status"];
	        this.progress_pct = source["progress_pct"];
	    }
	}
	export class NotebookSummary {
	    notebook_id: string;
	    name: string;
	    documents: number;
	
	    static createFrom(source: any = {}) {
	        return new NotebookSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notebook_id = source["notebook_id"];
	        this.name = source["name"];
	        this.documents = source["documents"];
	    }
	}
	export class DashboardSnapshot {
	    due_today: number;
	    study_streak_days: number;
	    active_notebooks: number;
	    pending_sync: number;
	    notebooks: NotebookSummary[];
	    ingestion: IngestionStatusRow[];
	    sync_status_text: string;
	    generated_at_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new DashboardSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.due_today = source["due_today"];
	        this.study_streak_days = source["study_streak_days"];
	        this.active_notebooks = source["active_notebooks"];
	        this.pending_sync = source["pending_sync"];
	        this.notebooks = this.convertValues(source["notebooks"], NotebookSummary);
	        this.ingestion = this.convertValues(source["ingestion"], IngestionStatusRow);
	        this.sync_status_text = source["sync_status_text"];
	        this.generated_at_ms = source["generated_at_ms"];
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

