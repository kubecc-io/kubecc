import Vue from 'vue';
import Vuetify from 'vuetify/lib/framework';
// import colors from 'vuetify/lib/util/colors'

Vue.use(Vuetify);

export default new Vuetify({
  theme: { dark: true },
  customVariables: ["~/styles/variables.scss"],
  treeShake: true,
  // theme: {
  //   dark: true,
  //   themes: {
  //     light: {
  //       primary: colors.blue, 
  //       secondary: colors.orange,
  //     },
  //     dark: {
  //       primary: colors.blue.lighten2, 
  //       secondary: colors.orange.lighten2, 
  //     },
  //   }
  // }
});
