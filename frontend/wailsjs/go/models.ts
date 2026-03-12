export namespace main {
	
	export class AppStatus {
	    xboxDetected: boolean;
	    xboxMAC: string;
	    tunnelActive: boolean;
	    connected: boolean;
	    peerCount: number;
	    localIP: string;
	    publicIP: string;
	    interface: string;
	    gamertag: string;
	    serverPing: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.xboxDetected = source["xboxDetected"];
	        this.xboxMAC = source["xboxMAC"];
	        this.tunnelActive = source["tunnelActive"];
	        this.connected = source["connected"];
	        this.peerCount = source["peerCount"];
	        this.localIP = source["localIP"];
	        this.publicIP = source["publicIP"];
	        this.interface = source["interface"];
	        this.gamertag = source["gamertag"];
	        this.serverPing = source["serverPing"];
	        this.error = source["error"];
	    }
	}
	export class PlayerInfo {
	    name: string;
	    ping: number;
	    isHost: boolean;
	    isYou: boolean;
	    connected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlayerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ping = source["ping"];
	        this.isHost = source["isHost"];
	        this.isYou = source["isYou"];
	        this.connected = source["connected"];
	    }
	}
	export class LobbyInfo {
	    id: string;
	    name: string;
	    game: string;
	    host: string;
	    players: number;
	    maxPlayers: number;
	    ping: number;
	    region: string;
	    code: string;
	    hostPublicIP: string;
	    hostPort: number;
	    members: PlayerInfo[];
	
	    static createFrom(source: any = {}) {
	        return new LobbyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.game = source["game"];
	        this.host = source["host"];
	        this.players = source["players"];
	        this.maxPlayers = source["maxPlayers"];
	        this.ping = source["ping"];
	        this.region = source["region"];
	        this.code = source["code"];
	        this.hostPublicIP = source["hostPublicIP"];
	        this.hostPort = source["hostPort"];
	        this.members = this.convertValues(source["members"], PlayerInfo);
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
	export class NetworkInterface {
	    name: string;
	    ip: string;
	    mac: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.mac = source["mac"];
	        this.description = source["description"];
	    }
	}
	export class PermissionStatus {
	    ok: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	    }
	}
	
	export class PortForwardInfo {
	    publicIP: string;
	    localIP: string;
	    port: number;
	    gatewayIP: string;
	    upnpStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new PortForwardInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.publicIP = source["publicIP"];
	        this.localIP = source["localIP"];
	        this.port = source["port"];
	        this.gatewayIP = source["gatewayIP"];
	        this.upnpStatus = source["upnpStatus"];
	    }
	}

}

