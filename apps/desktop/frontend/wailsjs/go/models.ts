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

