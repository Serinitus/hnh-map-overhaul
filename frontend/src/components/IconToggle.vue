<template>
  <v-icon :color="value ? onColor : offColor" @click="$emit('input', !value)"
          style="cursor:pointer;" :title="title">
    {{ value ? onIcon : offIcon }}
  </v-icon>
</template>

<script>
export default {
  name: "IconToggle",
  props: {
    value: {type: Boolean, required: true},
    // Defaults are the show/hide eye -- pass onIcon/offIcon to reuse this
    // for a different pair (e.g. the name-visibility checkmark/X bubble).
    onIcon: {type: String, default: 'mdi-eye'},
    offIcon: {type: String, default: 'mdi-eye-off'},
    onColor: {type: String, default: 'green'},
    offColor: {type: String, default: 'red'},
    title: {type: String, default: ''},
  },
}
</script>
