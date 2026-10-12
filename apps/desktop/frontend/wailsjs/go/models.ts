export namespace contracts {

	export class Measurement {
	    id: number;
	    gateway_id: string;
	    gateway_session_id: string;
	    event_seq: number;
	    gateway_rx_uptime_ms: number;
	    // Go type: time
	    gateway_received_at?: any;
	    packet_version: number;
	    device_id: string;
	    device_sequence?: number;
	    state: string;
	    diag_code?: number;
	    rms_g?: number;
	    temperature_c?: number;
	    rssi_dbm?: number;
	    snr_db?: number;
	    packet_crc16?: number;
	    // Go type: time
	    ingested_at: any;
	    // Go type: time
	    estimated_received_at?: any;
	    queue_delay_ms: number;

	    static createFrom(source: any = {}) {
	        return new Measurement(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gateway_id = source["gateway_id"];
	        this.gateway_session_id = source["gateway_session_id"];
	        this.event_seq = source["event_seq"];
	        this.gateway_rx_uptime_ms = source["gateway_rx_uptime_ms"];
	        this.gateway_received_at = this.convertValues(source["gateway_received_at"], null);
	        this.packet_version = source["packet_version"];
	        this.device_id = source["device_id"];
	        this.device_sequence = source["device_sequence"];
	        this.state = source["state"];
	        this.diag_code = source["diag_code"];
	        this.rms_g = source["rms_g"];
	        this.temperature_c = source["temperature_c"];
	        this.rssi_dbm = source["rssi_dbm"];
	        this.snr_db = source["snr_db"];
	        this.packet_crc16 = source["packet_crc16"];
	        this.ingested_at = this.convertValues(source["ingested_at"], null);
	        this.estimated_received_at = this.convertValues(source["estimated_received_at"], null);
	        this.queue_delay_ms = source["queue_delay_ms"];
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
	export class Device {
	    id: string;
	    profile: string;
	    notes: string;
	    motor_id?: string;
	    // Go type: time
	    created_at: any;
	    latest?: Measurement;

	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profile = source["profile"];
	        this.notes = source["notes"];
	        this.motor_id = source["motor_id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.latest = this.convertValues(source["latest"], Measurement);
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
	export class HeartbeatCounters {
	    lora_valid: number;
	    lora_rejected: number;
	    spi_errors: number;
	    http_failed_attempts: number;
	    readings_acknowledged: number;
	    queue_dropped: number;

	    static createFrom(source: any = {}) {
	        return new HeartbeatCounters(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lora_valid = source["lora_valid"];
	        this.lora_rejected = source["lora_rejected"];
	        this.spi_errors = source["spi_errors"];
	        this.http_failed_attempts = source["http_failed_attempts"];
	        this.readings_acknowledged = source["readings_acknowledged"];
	        this.queue_dropped = source["queue_dropped"];
	    }
	}
	export class HeartbeatHealth {
	    radio_initialized: boolean;
	    wifi_connected: boolean;
	    ntp_synchronized: boolean;
	    queue_size: number;
	    queue_capacity: number;
	    free_heap_bytes: number;
	    wifi_rssi_dbm?: number;

	    static createFrom(source: any = {}) {
	        return new HeartbeatHealth(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.radio_initialized = source["radio_initialized"];
	        this.wifi_connected = source["wifi_connected"];
	        this.ntp_synchronized = source["ntp_synchronized"];
	        this.queue_size = source["queue_size"];
	        this.queue_capacity = source["queue_capacity"];
	        this.free_heap_bytes = source["free_heap_bytes"];
	        this.wifi_rssi_dbm = source["wifi_rssi_dbm"];
	    }
	}
	export class Heartbeat {
	    schema_version: number;
	    gateway_id: string;
	    gateway_session_id: string;
	    heartbeat_seq: number;
	    uptime_ms: number;
	    // Go type: time
	    sent_at?: any;
	    firmware_version: string;
	    health: HeartbeatHealth;
	    counters: HeartbeatCounters;

	    static createFrom(source: any = {}) {
	        return new Heartbeat(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.gateway_id = source["gateway_id"];
	        this.gateway_session_id = source["gateway_session_id"];
	        this.heartbeat_seq = source["heartbeat_seq"];
	        this.uptime_ms = source["uptime_ms"];
	        this.sent_at = this.convertValues(source["sent_at"], null);
	        this.firmware_version = source["firmware_version"];
	        this.health = this.convertValues(source["health"], HeartbeatHealth);
	        this.counters = this.convertValues(source["counters"], HeartbeatCounters);
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
	export class Gateway {
	    id: string;
	    name: string;
	    location: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    last_ingested_at?: any;
	    // Go type: time
	    last_heartbeat_at?: any;
	    heartbeat?: Heartbeat;
	    active_session_id?: string;
	    activity: string;

	    static createFrom(source: any = {}) {
	        return new Gateway(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.location = source["location"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.last_ingested_at = this.convertValues(source["last_ingested_at"], null);
	        this.last_heartbeat_at = this.convertValues(source["last_heartbeat_at"], null);
	        this.heartbeat = this.convertValues(source["heartbeat"], Heartbeat);
	        this.active_session_id = source["active_session_id"];
	        this.activity = source["activity"];
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
	export class StoredGatewayLog {
	    id: string;
	    gateway_id: string;
	    gateway_session_id: string;
	    log_seq: number;
	    uptime_ms: number;
	    level: string;
	    event: string;
	    message: string;
	    truncated: boolean;
	    // Go type: time
	    received_at: any;

	    static createFrom(source: any = {}) {
	        return new StoredGatewayLog(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gateway_id = source["gateway_id"];
	        this.gateway_session_id = source["gateway_session_id"];
	        this.log_seq = source["log_seq"];
	        this.uptime_ms = source["uptime_ms"];
	        this.level = source["level"];
	        this.event = source["event"];
	        this.message = source["message"];
	        this.truncated = source["truncated"];
	        this.received_at = this.convertValues(source["received_at"], null);
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
	export class GatewayLogPage {
	    items: StoredGatewayLog[];
	    next_cursor: string;
	    has_more: boolean;

	    static createFrom(source: any = {}) {
	        return new GatewayLogPage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], StoredGatewayLog);
	        this.next_cursor = source["next_cursor"];
	        this.has_more = source["has_more"];
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



	export class HistoryPage {
	    items: Measurement[];
	    next_cursor?: string;

	    static createFrom(source: any = {}) {
	        return new HistoryPage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Measurement);
	        this.next_cursor = source["next_cursor"];
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

	export class Motor {
	    id: string;
	    name: string;
	    location: string;
	    device_id?: string;
	    expected_interval_seconds: number;
	    // Go type: time
	    created_at: any;
	    latest?: Measurement;

	    static createFrom(source: any = {}) {
	        return new Motor(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.location = source["location"];
	        this.device_id = source["device_id"];
	        this.expected_interval_seconds = source["expected_interval_seconds"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.latest = this.convertValues(source["latest"], Measurement);
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
	export class MotorInput {
	    id: string;
	    name: string;
	    location: string;
	    device_id?: string;
	    expected_interval_seconds: number;

	    static createFrom(source: any = {}) {
	        return new MotorInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.location = source["location"];
	        this.device_id = source["device_id"];
	        this.expected_interval_seconds = source["expected_interval_seconds"];
	    }
	}

}

export namespace main {

	export class ConnectionInfo {
	    endpoint: string;
	    email: string;
	    authenticated: boolean;
	    expiresAt: string;

	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.email = source["email"];
	        this.authenticated = source["authenticated"];
	        this.expiresAt = source["expiresAt"];
	    }
	}
	export class Workspace {
	    motors: contracts.Motor[];
	    devices: contracts.Device[];
	    gateways: contracts.Gateway[];
	    recentReadings: contracts.Measurement[];
	    fetchedAt: string;

	    static createFrom(source: any = {}) {
	        return new Workspace(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.motors = this.convertValues(source["motors"], contracts.Motor);
	        this.devices = this.convertValues(source["devices"], contracts.Device);
	        this.gateways = this.convertValues(source["gateways"], contracts.Gateway);
	        this.recentReadings = this.convertValues(source["recentReadings"], contracts.Measurement);
	        this.fetchedAt = source["fetchedAt"];
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

export namespace store {

	export class Profile {
	    name: string;
	    email: string;
	    company: string;
	    jobTitle: string;
	    avatarData: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.email = source["email"];
	        this.company = source["company"];
	        this.jobTitle = source["jobTitle"];
	        this.avatarData = source["avatarData"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

