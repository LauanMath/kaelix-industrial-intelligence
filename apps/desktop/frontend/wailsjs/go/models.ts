export namespace contracts {
	
	export class Collection {
	    id: number;
	    name: string;
	    device_id: string;
	    condition: string;
	    notes: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    ended_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.device_id = source["device_id"];
	        this.condition = source["condition"];
	        this.notes = source["notes"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.ended_at = this.convertValues(source["ended_at"], null);
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
	export class CollectionInput {
	    name: string;
	    device_id: string;
	    condition: string;
	    notes: string;
	    // Go type: time
	    started_at?: any;
	    // Go type: time
	    ended_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new CollectionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.device_id = source["device_id"];
	        this.condition = source["condition"];
	        this.notes = source["notes"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.ended_at = this.convertValues(source["ended_at"], null);
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
	export class Range {
	    min: number;
	    avg: number;
	    max: number;
	
	    static createFrom(source: any = {}) {
	        return new Range(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min = source["min"];
	        this.avg = source["avg"];
	        this.max = source["max"];
	    }
	}
	export class CollectionSummary {
	    collection: Collection;
	    // Go type: time
	    from: any;
	    // Go type: time
	    to: any;
	    readings: number;
	    readings_without_measurement: number;
	    readings_saturated: number;
	    readings_undated: number;
	    sequence_gaps: number;
	    sequence_missing: number;
	    waveforms: number;
	    waveforms_incomplete: number;
	    waveforms_undated: number;
	    fragments_total: number;
	    fragments_missing: number;
	    rssi_dbm?: Range;
	    snr_db?: Range;
	
	    static createFrom(source: any = {}) {
	        return new CollectionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.collection = this.convertValues(source["collection"], Collection);
	        this.from = this.convertValues(source["from"], null);
	        this.to = this.convertValues(source["to"], null);
	        this.readings = source["readings"];
	        this.readings_without_measurement = source["readings_without_measurement"];
	        this.readings_saturated = source["readings_saturated"];
	        this.readings_undated = source["readings_undated"];
	        this.sequence_gaps = source["sequence_gaps"];
	        this.sequence_missing = source["sequence_missing"];
	        this.waveforms = source["waveforms"];
	        this.waveforms_incomplete = source["waveforms_incomplete"];
	        this.waveforms_undated = source["waveforms_undated"];
	        this.fragments_total = source["fragments_total"];
	        this.fragments_missing = source["fragments_missing"];
	        this.rssi_dbm = this.convertValues(source["rssi_dbm"], Range);
	        this.snr_db = this.convertValues(source["snr_db"], Range);
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
	    boot_count?: number;
	    command_seq?: number;
	    rms_x_g?: number;
	    rms_y_g?: number;
	    kurtosis?: number;
	    crest_factor?: number;
	    dominant_freq_hz?: number;
	    sample_rate_hz?: number;
	    config_version?: number;
	    reset_reason?: number;
	    flags?: number;
	    // Go type: time
	    device_time?: any;
	    uptime_s?: number;
	    wave_fragments?: number;
	    wave_samples?: number;
	    channels?: number;
	    tx_power_dbm?: number;
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
	        this.boot_count = source["boot_count"];
	        this.command_seq = source["command_seq"];
	        this.rms_x_g = source["rms_x_g"];
	        this.rms_y_g = source["rms_y_g"];
	        this.kurtosis = source["kurtosis"];
	        this.crest_factor = source["crest_factor"];
	        this.dominant_freq_hz = source["dominant_freq_hz"];
	        this.sample_rate_hz = source["sample_rate_hz"];
	        this.config_version = source["config_version"];
	        this.reset_reason = source["reset_reason"];
	        this.flags = source["flags"];
	        this.device_time = this.convertValues(source["device_time"], null);
	        this.uptime_s = source["uptime_s"];
	        this.wave_fragments = source["wave_fragments"];
	        this.wave_samples = source["wave_samples"];
	        this.channels = source["channels"];
	        this.tx_power_dbm = source["tx_power_dbm"];
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
	export class DeviceCommand {
	    device_id: string;
	    seq: number;
	    kind: string;
	    state: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    radiated_at?: any;
	    // Go type: time
	    applied_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new DeviceCommand(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.seq = source["seq"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.radiated_at = this.convertValues(source["radiated_at"], null);
	        this.applied_at = this.convertValues(source["applied_at"], null);
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
	export class RoutineWindow {
	    days: number;
	    start_min: number;
	    end_min: number;
	
	    static createFrom(source: any = {}) {
	        return new RoutineWindow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.days = source["days"];
	        this.start_min = source["start_min"];
	        this.end_min = source["end_min"];
	    }
	}
	export class DeviceConfig {
	    version: number;
	    cycle_interval_s: number;
	    window_samples: number;
	    channels: number;
	    accel_range: number;
	    gyro_range: number;
	    dlpf: number;
	    tx_power_dbm: number;
	    burst_profile: number;
	    wave_every: number;
	    link_loss_s: number;
	    checkin_interval_s: number;
	    tz_offset_min: number;
	    routine: RoutineWindow[];
	
	    static createFrom(source: any = {}) {
	        return new DeviceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.cycle_interval_s = source["cycle_interval_s"];
	        this.window_samples = source["window_samples"];
	        this.channels = source["channels"];
	        this.accel_range = source["accel_range"];
	        this.gyro_range = source["gyro_range"];
	        this.dlpf = source["dlpf"];
	        this.tx_power_dbm = source["tx_power_dbm"];
	        this.burst_profile = source["burst_profile"];
	        this.wave_every = source["wave_every"];
	        this.link_loss_s = source["link_loss_s"];
	        this.checkin_interval_s = source["checkin_interval_s"];
	        this.tz_offset_min = source["tz_offset_min"];
	        this.routine = this.convertValues(source["routine"], RoutineWindow);
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
	export class ErrorDetail {
	    path: string;
	    reason: string;
	    minimum?: number;
	
	    static createFrom(source: any = {}) {
	        return new ErrorDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.reason = source["reason"];
	        this.minimum = source["minimum"];
	    }
	}
	export class DeviceConfigCheck {
	    issues: ErrorDetail[];
	    cycle_seconds?: number;
	    minimum_cycle_s?: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceConfigCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issues = this.convertValues(source["issues"], ErrorDetail);
	        this.cycle_seconds = source["cycle_seconds"];
	        this.minimum_cycle_s = source["minimum_cycle_s"];
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
	export class DeviceConfigState {
	    device_id: string;
	    config?: DeviceConfig;
	    state?: string;
	    radiated_version?: number;
	    reported_version?: number;
	    reported_command_seq?: number;
	    // Go type: time
	    updated_at?: any;
	    // Go type: time
	    radiated_at?: any;
	    // Go type: time
	    reported_at?: any;
	    // Go type: time
	    applied_at?: any;
	    defaults: DeviceConfig;
	
	    static createFrom(source: any = {}) {
	        return new DeviceConfigState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.config = this.convertValues(source["config"], DeviceConfig);
	        this.state = source["state"];
	        this.radiated_version = source["radiated_version"];
	        this.reported_version = source["reported_version"];
	        this.reported_command_seq = source["reported_command_seq"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.radiated_at = this.convertValues(source["radiated_at"], null);
	        this.reported_at = this.convertValues(source["reported_at"], null);
	        this.applied_at = this.convertValues(source["applied_at"], null);
	        this.defaults = this.convertValues(source["defaults"], DeviceConfig);
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
	    spool_dropped?: number;
	    waveforms_acknowledged?: number;
	
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
	        this.spool_dropped = source["spool_dropped"];
	        this.waveforms_acknowledged = source["waveforms_acknowledged"];
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
	    spool_used_bytes?: number;
	    spool_capacity_bytes?: number;
	    ssid?: string;
	    ip?: string;
	    reset_reason?: string;
	
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
	        this.spool_used_bytes = source["spool_used_bytes"];
	        this.spool_capacity_bytes = source["spool_capacity_bytes"];
	        this.ssid = source["ssid"];
	        this.ip = source["ip"];
	        this.reset_reason = source["reset_reason"];
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
	export class WifiCredential {
	    ssid: string;
	    password?: string;
	
	    static createFrom(source: any = {}) {
	        return new WifiCredential(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssid = source["ssid"];
	        this.password = source["password"];
	    }
	}
	export class GatewayConfigInput {
	    version?: number;
	    wifi: WifiCredential[];
	    heartbeat_interval_s: number;
	    sync_interval_s: number;
	    ap_when_offline: boolean;
	    tx_power_dbm: number;
	
	    static createFrom(source: any = {}) {
	        return new GatewayConfigInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.wifi = this.convertValues(source["wifi"], WifiCredential);
	        this.heartbeat_interval_s = source["heartbeat_interval_s"];
	        this.sync_interval_s = source["sync_interval_s"];
	        this.ap_when_offline = source["ap_when_offline"];
	        this.tx_power_dbm = source["tx_power_dbm"];
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
	export class WifiStatus {
	    ssid: string;
	    has_password: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WifiStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssid = source["ssid"];
	        this.has_password = source["has_password"];
	    }
	}
	export class GatewayConfigView {
	    version: number;
	    wifi: WifiStatus[];
	    heartbeat_interval_s: number;
	    sync_interval_s: number;
	    ap_when_offline: boolean;
	    tx_power_dbm: number;
	
	    static createFrom(source: any = {}) {
	        return new GatewayConfigView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.wifi = this.convertValues(source["wifi"], WifiStatus);
	        this.heartbeat_interval_s = source["heartbeat_interval_s"];
	        this.sync_interval_s = source["sync_interval_s"];
	        this.ap_when_offline = source["ap_when_offline"];
	        this.tx_power_dbm = source["tx_power_dbm"];
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
	export class GatewayConfigState {
	    gateway_id: string;
	    config?: GatewayConfigView;
	    state?: string;
	    reported_version?: number;
	    // Go type: time
	    updated_at?: any;
	    // Go type: time
	    reported_at?: any;
	    // Go type: time
	    applied_at?: any;
	    // Go type: time
	    last_sync_at?: any;
	    defaults: GatewayConfigView;
	
	    static createFrom(source: any = {}) {
	        return new GatewayConfigState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gateway_id = source["gateway_id"];
	        this.config = this.convertValues(source["config"], GatewayConfigView);
	        this.state = source["state"];
	        this.reported_version = source["reported_version"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.reported_at = this.convertValues(source["reported_at"], null);
	        this.applied_at = this.convertValues(source["applied_at"], null);
	        this.last_sync_at = this.convertValues(source["last_sync_at"], null);
	        this.defaults = this.convertValues(source["defaults"], GatewayConfigView);
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
	
	export class GatewayFirmware {
	    id: string;
	    gateway_id: string;
	    version: string;
	    sha256: string;
	    size: number;
	    state: string;
	    reason: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    offered_at?: any;
	    // Go type: time
	    downloaded_at?: any;
	    // Go type: time
	    finished_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new GatewayFirmware(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gateway_id = source["gateway_id"];
	        this.version = source["version"];
	        this.sha256 = source["sha256"];
	        this.size = source["size"];
	        this.state = source["state"];
	        this.reason = source["reason"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.offered_at = this.convertValues(source["offered_at"], null);
	        this.downloaded_at = this.convertValues(source["downloaded_at"], null);
	        this.finished_at = this.convertValues(source["finished_at"], null);
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
	export class GatewayFirmwareStatus {
	    gateway_id: string;
	    current_version?: string;
	    items: GatewayFirmware[];
	
	    static createFrom(source: any = {}) {
	        return new GatewayFirmwareStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gateway_id = source["gateway_id"];
	        this.current_version = source["current_version"];
	        this.items = this.convertValues(source["items"], GatewayFirmware);
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
	
	
	
	export class Waveform {
	    id: number;
	    gateway_id: string;
	    gateway_session_id: string;
	    event_seq: number;
	    device_id: string;
	    gateway_rx_uptime_ms: number;
	    // Go type: time
	    gateway_received_at?: any;
	    device_sequence: number;
	    boot_count: number;
	    channels: number;
	    window_samples: number;
	    sample_rate_hz: number;
	    accel_range: number;
	    gyro_range: number;
	    dlpf: number;
	    burst_profile: number;
	    fragment_total: number;
	    fragments_missing: number[];
	    samples_sha256: string;
	    // Go type: time
	    ingested_at: any;
	    // Go type: time
	    estimated_received_at?: any;
	    queue_delay_ms: number;
	    samples_b64?: string;
	
	    static createFrom(source: any = {}) {
	        return new Waveform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gateway_id = source["gateway_id"];
	        this.gateway_session_id = source["gateway_session_id"];
	        this.event_seq = source["event_seq"];
	        this.device_id = source["device_id"];
	        this.gateway_rx_uptime_ms = source["gateway_rx_uptime_ms"];
	        this.gateway_received_at = this.convertValues(source["gateway_received_at"], null);
	        this.device_sequence = source["device_sequence"];
	        this.boot_count = source["boot_count"];
	        this.channels = source["channels"];
	        this.window_samples = source["window_samples"];
	        this.sample_rate_hz = source["sample_rate_hz"];
	        this.accel_range = source["accel_range"];
	        this.gyro_range = source["gyro_range"];
	        this.dlpf = source["dlpf"];
	        this.burst_profile = source["burst_profile"];
	        this.fragment_total = source["fragment_total"];
	        this.fragments_missing = source["fragments_missing"];
	        this.samples_sha256 = source["samples_sha256"];
	        this.ingested_at = this.convertValues(source["ingested_at"], null);
	        this.estimated_received_at = this.convertValues(source["estimated_received_at"], null);
	        this.queue_delay_ms = source["queue_delay_ms"];
	        this.samples_b64 = source["samples_b64"];
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
	export class WaveformPage {
	    items: Waveform[];
	    next_cursor?: string;
	
	    static createFrom(source: any = {}) {
	        return new WaveformPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Waveform);
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
	

}

export namespace gatewaylink {
	
	export class Info {
	    mac: string;
	    firmware_version: string;
	    gateway_id: string;
	    paired: boolean;
	    connected: boolean;
	    ssid: string;
	    ip: string;
	    config_version: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mac = source["mac"];
	        this.firmware_version = source["firmware_version"];
	        this.gateway_id = source["gateway_id"];
	        this.paired = source["paired"];
	        this.connected = source["connected"];
	        this.ssid = source["ssid"];
	        this.ip = source["ip"];
	        this.config_version = source["config_version"];
	    }
	}
	export class Network {
	    ssid: string;
	    rssi: number;
	    secure: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Network(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssid = source["ssid"];
	        this.rssi = source["rssi"];
	        this.secure = source["secure"];
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
	export class EndpointProbe {
	    endpoint: string;
	    reachable: boolean;
	    localEndpoint: string;
	    localReachable: boolean;
	    suggestLocal: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EndpointProbe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.reachable = source["reachable"];
	        this.localEndpoint = source["localEndpoint"];
	        this.localReachable = source["localReachable"];
	        this.suggestLocal = source["suggestLocal"];
	    }
	}
	export class ExportResult {
	    folder: string;
	    readings: number;
	    windows: number;
	    readingsUndated: number;
	    windowsUndated: number;
	
	    static createFrom(source: any = {}) {
	        return new ExportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = source["folder"];
	        this.readings = source["readings"];
	        this.windows = source["windows"];
	        this.readingsUndated = source["readingsUndated"];
	        this.windowsUndated = source["windowsUndated"];
	    }
	}
	export class ProvisionStatus {
	    gatewayId: string;
	    existing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProvisionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gatewayId = source["gatewayId"];
	        this.existing = source["existing"];
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

