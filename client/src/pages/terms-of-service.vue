<template>
  <v-container
    class="pa-2"
    max-width="1400"
  >
    <v-card class="elevation-3 rounded-lg">
      <v-card-title class="text-center grey--text text-h4 font-weight-bold">
        Terms of Service
      </v-card-title>
      <v-card-text>
        <v-list>
          <v-list-item-group
            v-for="(section, index) in sections"
            :key="index"
          >
            <v-list-item class="mt-5">
              <div>
                <h3 class="headline font-weight-bold">
                  {{ section.title }}
                </h3>
                <p>{{ section.content }}</p>
              </div>
            </v-list-item>
          </v-list-item-group>
        </v-list>
      </v-card-text>
    </v-card>
  </v-container>
</template>

<script lang="js">
export default {
  data() {
    return {
      sections: [
        {
          title: 'Acceptance of Terms',
          content: 'By using the service, you agree to comply with and be bound by these Terms of Service. If you do not agree, do not use the application.'
        },
        {
          title: 'Description of Service',
          content: 'The bot collects message content data from channels it has access to, using that data for training a Markov chain for text generation. Users must use the /train command to let the bot fetch messages.'
        },
        {
          title: 'Data Collection',
          content: 'The bot collects message content data, excluding user data or timestamps. It saves media URLs, including images, GIFs, and videos, even NSFW ones.'
        },
        {
          title: 'User Responsibilities',
          content: 'You are responsible for ensuring the bot has the correct permissions to access channels. You agree not to misuse the bot for illegal activities.'
        },
        {
          title: 'Prohibited Conduct',
          content: 'No spamming, phishing, malicious activities, harassment, hate speech, or any other harmful behavior. Do not use automated scripts to interact with the bot inappropriately.'
        },
        {
          title: 'Open Source and Usage',
          content: 'The bot\'s code is open source, and we encourage free and open use.'
        },
        {
          title: 'Termination',
          content: 'We reserve the right to suspend or terminate your access at any time, without notice, if your conduct violates these Terms of Service or harms others.'
        },
        {
          title: 'Limitation of Liability',
          content: 'The bot is provided "as is" without warranties of any kind. We are not liable for any damages, including loss of profits, goodwill, use, or data.'
        },
        {
          title: 'Amendments',
          content: 'We reserve the right to modify these Terms of Service at any time. Changes will be effective immediately upon posting. Continued use of the application constitutes acceptance of the modified terms.'
        },
        {
          title: 'Contact Information',
          content: 'For questions about these Terms of Service, contact the owner "zlejon" on Discord via the official Discord server.'
        }
      ]
    };
  }
};
</script>
