<script setup lang="ts">
import { onLaunch } from "@dcloudio/uni-app";
import { initCloud } from "@/utils/request";

onLaunch(() => {
  initCloud();
});
</script>

<style lang="scss">
@import "@/styles/tokens.scss";

page {
  background: $hm-bg;
  color: $hm-text;
  font-family: "PingFang SC", "HarmonyOS Sans SC", "Noto Sans SC", sans-serif;
}
</style>
