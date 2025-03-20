<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="chart-wrapper">
    <apex-chart
      :height="200"
      :width="200"
      type="radialBar"
      :options="options"
      :series="[percentage]"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, toRefs } from "vue";
import ApexChart from "vue3-apexcharts";
import { primary, naturals } from "@/vars/colors";
import { ApexOptions } from "apexcharts";

const props = defineProps<{ chartFillPercents:  number | string }>();

const { chartFillPercents } = toRefs(props);
const options: ApexOptions = {
  chart: {
    dropShadow: {
      enabled: true,
      top: 0,
      left: 0,
      blur: 5,
      color: primary.P3,
      opacity: 0.2,
    },
  },
  plotOptions: {
    radialBar: {
      hollow: {
        margin: 0,
        size: "60",
      },
      track: {
        background: naturals.N0
      },
      dataLabels: {
        show: false,
      },
    },
  },
  fill: {
    colors: [primary.P3],
  },
  stroke: {
    lineCap: "round"
  },
  states: {
    hover: {
      filter: {
        type: 'none',
      }
    },
    active: {
      filter: {
        type: 'none',
      }
    }
  }
};

const percentage = computed(() => chartFillPercents.value ?? 0);
</script>

<style scoped>
.chart-wrapper {
  @apply flex items-center justify-start z-0;
}
</style>
