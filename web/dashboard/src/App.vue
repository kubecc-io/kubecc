<template>
	<v-app>
		<v-navigation-drawer width="250" absolute permanent app>
			<template v-slot:prepend>
				<v-list-item two-line>
					<v-list-item-avatar tile="true" width="46">
						<img src="./assets/icon.png" />
					</v-list-item-avatar>

					<v-list-item-content>
						<v-list-item-title>kubecc</v-list-item-title>
						<v-list-item-subtitle>System Dashboard</v-list-item-subtitle>
					</v-list-item-content>
				</v-list-item>
			</template>

			<v-divider></v-divider>

			<v-list nav dense>
				<v-list-item-group v-model="selectedItem" color="primary">
					<v-list-item v-for="(item, i) in items" :key="i" :to="item.route">
						<v-list-item-icon>
							<v-icon
								v-text="i === selectedItem ? item.iconSelected : item.icon"
							></v-icon>
						</v-list-item-icon>

						<v-list-item-content>
							<v-list-item-title v-text="item.text"></v-list-item-title>
						</v-list-item-content>
					</v-list-item>
				</v-list-item-group>
			</v-list>

			<template v-slot:append>
				<v-divider></v-divider>
				<v-list-item two-line>
					<v-row align="center" justify="center">
						<v-switch v-model="$vuetify.theme.dark" label="Dark Mode" inset />
					</v-row>
				</v-list-item>
			</template>
		</v-navigation-drawer>
		<v-content>
			<v-container fluid fill-height>
				<v-fade-transition mode="out-in">
					<router-view></router-view>
				</v-fade-transition>
			</v-container>
		</v-content>
	</v-app>
</template>

<script>
export default {
	data: () => ({
		selectedItem: 0,
		items: [
			{
				text: "Status",
				icon: "mdi-view-dashboard-outline",
				iconSelected: "mdi-view-dashboard",
				route: "/",
			},
			{
				text: "Settings",
				icon: "mdi-cog-outline",
				iconSelected: "mdi-cog",
				route: "/settings",
			},
			{
				text: "About",
				icon: "mdi-information-outline",
				iconSelected: "mdi-information",
				route: "/about",
			},
		],
	}),
};
</script>
<style>
@font-face {
	font-family: Poppins;
	src: url(./assets/Poppins-Regular.ttf);
}

.body,
.v-application {
	font-family: Poppins;
}
</style>