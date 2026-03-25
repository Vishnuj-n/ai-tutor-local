export namespace main {
	
	export class CloudHealthProbeResult {
	    url: string;
	    ok: boolean;
	    status_code: number;
	    message?: string;
	    latency_ms: number;
	    checked_at: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudHealthProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.ok = source["ok"];
	        this.status_code = source["status_code"];
	        this.message = source["message"];
	        this.latency_ms = source["latency_ms"];
	        this.checked_at = source["checked_at"];
	    }
	}
	export class ReviewCardDTO {
	    flashcard_id: string;
	    notebook_id: string;
	    notebook_name: string;
	    question: string;
	    answer: string;
	    state: string;
	    due_at?: string;
	    reps: number;
	    lapses: number;
	    queue_position: number;
	    queue_size: number;
	
	    static createFrom(source: any = {}) {
	        return new ReviewCardDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flashcard_id = source["flashcard_id"];
	        this.notebook_id = source["notebook_id"];
	        this.notebook_name = source["notebook_name"];
	        this.question = source["question"];
	        this.answer = source["answer"];
	        this.state = source["state"];
	        this.due_at = source["due_at"];
	        this.reps = source["reps"];
	        this.lapses = source["lapses"];
	        this.queue_position = source["queue_position"];
	        this.queue_size = source["queue_size"];
	    }
	}
	export class ReviewRateInput {
	    flashcard_id: string;
	    notebook_id: string;
	    notebook_name: string;
	    rating: number;
	    time_taken_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ReviewRateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flashcard_id = source["flashcard_id"];
	        this.notebook_id = source["notebook_id"];
	        this.notebook_name = source["notebook_name"];
	        this.rating = source["rating"];
	        this.time_taken_ms = source["time_taken_ms"];
	    }
	}
	export class ReviewRateResult {
	    next_due_at: string;
	    state: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ReviewRateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.next_due_at = source["next_due_at"];
	        this.state = source["state"];
	        this.message = source["message"];
	    }
	}
	export class ReviewSessionSummaryInput {
	    notebook_id: string;
	    notebook_name: string;
	    started_at_ms: number;
	    ended_at_ms: number;
	    flashcards_reviewed: number;
	    correct_recall_count: number;
	    total_time_taken_ms: number;
	    emit_telemetry: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReviewSessionSummaryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notebook_id = source["notebook_id"];
	        this.notebook_name = source["notebook_name"];
	        this.started_at_ms = source["started_at_ms"];
	        this.ended_at_ms = source["ended_at_ms"];
	        this.flashcards_reviewed = source["flashcards_reviewed"];
	        this.correct_recall_count = source["correct_recall_count"];
	        this.total_time_taken_ms = source["total_time_taken_ms"];
	        this.emit_telemetry = source["emit_telemetry"];
	    }
	}
	export class SyncSettingsDTO {
	    base_url: string;
	    class_code: string;
	    student_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncSettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base_url = source["base_url"];
	        this.class_code = source["class_code"];
	        this.student_name = source["student_name"];
	    }
	}

}

export namespace sync {
	
	export class SyncStatus {
	    pending_count: number;
	    last_sync_time?: string;
	    health: string;
	    next_retry_in_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pending_count = source["pending_count"];
	        this.last_sync_time = source["last_sync_time"];
	        this.health = source["health"];
	        this.next_retry_in_ms = source["next_retry_in_ms"];
	    }
	}

}

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

