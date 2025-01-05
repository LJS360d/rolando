<template>
  <v-dialog v-model="visible" :max-width="425">
    <v-card>
      <v-card-title class="text-h6">{{ title }}</v-card-title>
      <v-card-text>{{ message }}</v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="cancel">Cancel</v-btn>
        <v-btn color="primary" @click="confirm">Confirm</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { defineComponent, ref, watch } from 'vue';

export default defineComponent({
  name: 'AppDialog',
  props: {
    title: {
      type: String,
      default: 'Confirm Action'
    },
    message: {
      type: String,
      default: 'Are you sure you want to proceed?'
    },
    modelValue: {
      type: Boolean,
      required: true
    }
  },
  emits: ['update:modelValue', 'confirm', 'cancel'],
  setup(props, { emit }) {
    const visible = ref(props.modelValue);

    watch(() => props.modelValue, (newVal) => {
      visible.value = newVal;
    });

    const cancel = () => {
      emit('cancel');
      emit('update:modelValue', false);
    };

    const confirm = async () => {
      emit('confirm');
      emit('update:modelValue', false);
    };

    return { visible, cancel, confirm };
  }
});
</script>
