<template>
  <v-container
    class="pa-2"
    max-width="1400"
  >
    <v-card class="elevation-3 rounded-lg">
      <v-card-title
        class="text-center grey--text text-h4 font-weight-bold"
      >
        Privacy Policy
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

<script>
export default {
    data() {
        return {
            sections: [
                {
                    title: "Introduction",
                    content:
                        "Welcome to the Privacy Policy. Your privacy is important to us. This policy explains what information we collect, how we use it, and your rights regarding that information.",
                },
                {
                    title: "Information We Collect",
                    content:
                        "We collect message content data from Discord channels. We do not collect personal user data, such as usernames or timestamps. Media URLs, including images, GIFs, and videos, even NSFW ones, are stored.",
                },
                {
                    title: "How We Use Collected Information",
                    content:
                        "The collected message content data is used to train a lightweight Markov chain for text generation tied to the server. Media URLs are stored for the bot's operational purposes. We do not share this data with third parties.",
                },
                {
                    title: "Data Storage and Security",
                    content:
                        "We implement reasonable security measures to protect the data collected. However, no method of transmission over the Internet or electronic storage is 100% secure.",
                },
                {
                    title: "User Controls and Data Deletion",
                    content:
                        'Users can manage what data the bot learns via commands. Users can also request the deletion of collected data by contacting the owner "zlejon" on Discord through the official Discord server.',
                },
                {
                    title: "Children's Privacy",
                    content:
                        "The bot is not intended for use by individuals under the age of 13. We do not knowingly collect personal data from children under 13.",
                },
                {
                    title: "Changes to This Privacy Policy",
                    content:
                        "We may update this Privacy Policy from time to time. We will notify users of any changes by posting the new Privacy Policy on this page.",
                },
                {
                    title: "Contact Us",
                    content:
                        'If you have any questions about this Privacy Policy, please contact the owner "zlejon" on Discord.',
                },
            ],
        };
    },
};
</script>
