<template>
	<v-container>
		<v-row>
			<v-col>
				<v-card elevation="12">
					<v-card-title>Core Components</v-card-title>
					<v-card-text>
						<v-container>
							<v-row cols="3">
								<v-col>
									<v-card elevation="4" color="success" dark>
										<v-card-title>Controller</v-card-title>
										<v-card-subtitle>Connected</v-card-subtitle>
										<v-card-actions>
											<v-btn color="primary">Restart</v-btn>
										</v-card-actions>
									</v-card>
								</v-col>
								<v-col>
									<v-card elevation="4" color="error" dark>
										<v-card-title>Scheduler</v-card-title>
										<v-card-subtitle>Not Connected</v-card-subtitle>
										<v-card-actions>
											<v-btn color="primary">Restart</v-btn>
										</v-card-actions>
									</v-card>
								</v-col>
								<v-col>
									<v-card elevation="4" color="success" dark>
										<v-card-title>Dashboard</v-card-title>
										<v-card-subtitle>Connected</v-card-subtitle>
										<v-card-actions>
											<v-btn color="primary">Restart</v-btn>
										</v-card-actions>
									</v-card>
								</v-col>
							</v-row>
						</v-container>
					</v-card-text>
				</v-card>
			</v-col>
		</v-row>
		<v-row cols="2">
			<v-col>
				<v-card elevation="12">
					<v-card-title>Agents</v-card-title>
					<v-card-text>
						<v-simple-table>
							<template v-slot:default>
								<thead>
									<tr>
										<th class="text-left">Hostname</th>
										<th class="text-left">Toolchains</th>
									</tr>
								</thead>
								<tbody>
									<tr v-for="item in agents" :key="item.hostname">
										<td>{{ item.hostname }}</td>
										<td>{{ item.toolchains }}</td>
									</tr>
								</tbody>
							</template>
						</v-simple-table>
					</v-card-text>
				</v-card>
			</v-col>
			<v-col>
				<v-card elevation="12">
					<v-card-title>Consumer Daemons</v-card-title>
					<v-card-text>
						<v-simple-table>
							<template v-slot:default>
								<thead>
									<tr>
										<th class="text-left">Hostname</th>
										<th class="text-left">Toolchains</th>
									</tr>
								</thead>
								<tbody>
									<tr v-for="item in agents" :key="item.hostname">
										<td>{{ item.hostname }}</td>
										<td>{{ item.toolchains }}</td>
									</tr>
								</tbody>
							</template>
						</v-simple-table>
					</v-card-text>
				</v-card>
			</v-col>
		</v-row>
	</v-container>
</template>

<script>
export default {
	name: "Status",
	data: () => ({
		loadingController: true,
		loadingAgents: true,
		loadingScheduler: true,
		loadingConsumerd: true,
		agents: [
			{
				hostname: "test-agent",
				toolchains: "x86_64_Gnu_CXX",
			},
		],
	}),
	methods: {
		querySystemStatus: function () {
			var vm = this;
			setInterval(function () {
				fetch("http://localhost:9091/api/status")
					.then((r) => r.json())
					.then(function (data) {
						console.log(data);
						for (const statusItem of data.StatusItems) {
							switch (statusItem.Component) {
								case 1: // Agent
									vm.loadingAgents = !statusItem.Alive;
									break;
								case 2: // Scheduler
									vm.loadingScheduler = !statusItem.Alive;
									break;
								case 3: // Controller
									vm.loadingController = !statusItem.Alive;
									break;
								case 4: // Consumer
									break;
								case 5: // Consumerd
									vm.loadingConsumerd = !statusItem.Alive;
									break;
								case 6: // Make
								case 7: // Test
								case 8: // Dashboard
									break;
							}
						}
					});
			}, 1000);
		},
	},
	beforeMount() {
		this.querySystemStatus();
	},
};
</script>
