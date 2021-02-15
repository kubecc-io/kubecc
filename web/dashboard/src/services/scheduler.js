import { SchedulerPromiseClient } from "../../proto/types_grpc_web_pb"
import { Empty } from "../../proto/types_pb"

export default class {
  constructor() {
    this.client = new SchedulerPromiseClient("http://localhost:9999", null, null)
  }

  async watchStatus() { 
    try {
			const stream = await this.client.watchSystemStatus(new Empty(), {});
			stream.on("data", function (response) {
				var systemStatus = response.getStatusItemsList();
				for (var item in systemStatus) {
					console.log(item.getComponent());
					console.log(item.getAlive());
				}
				console.log(response.getMessage());
			});
			stream.on("status", function (status) {
				console.log(status.code);
				console.log(status.details);
				console.log(status.metadata);
			});
			stream.on("end", function () {
				console.log("Stream ended");
			});
		} catch (err) {
			console.error(err.message);
			throw err;
		}
  }
}