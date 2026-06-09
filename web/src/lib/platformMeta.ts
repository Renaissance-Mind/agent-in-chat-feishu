export interface FieldDef {
  key: string;
  labelKey: string;
  required?: boolean;
  type?: 'text' | 'password' | 'number' | 'boolean';
  placeholder?: string;
  hintKey?: string;
  group?: 'basic' | 'advanced';
}

export interface PlatformMeta {
  label: string;
  fields: FieldDef[];
}

export const platformMeta: Record<string, PlatformMeta> = {
  feishu: {
    label: 'Feishu / Lark',
    fields: [
      { key: 'app_id', labelKey: 'fields.appId', required: true, placeholder: 'cli_xxx' },
      { key: 'app_secret', labelKey: 'fields.appSecret', required: true, type: 'password' },
      { key: 'encrypt_key', labelKey: 'fields.encryptKey', type: 'password', group: 'advanced' },
      { key: 'verification_token', labelKey: 'fields.verificationToken', type: 'password', group: 'advanced' },
      { key: 'allow_from', labelKey: 'fields.allowFrom', placeholder: '* (all)', group: 'advanced' },
      { key: 'group_reply_all', labelKey: 'fields.groupReplyAll', type: 'boolean', group: 'advanced' },
      { key: 'share_session_in_channel', labelKey: 'fields.sharedGroupSession', type: 'boolean', group: 'advanced' },
      { key: 'group_context_buffer', labelKey: 'fields.groupContextBuffer', type: 'boolean', group: 'advanced' },
    ],
  },
  lark: {
    label: 'Feishu / Lark',
    fields: [
      { key: 'app_id', labelKey: 'fields.appId', required: true, placeholder: 'cli_xxx' },
      { key: 'app_secret', labelKey: 'fields.appSecret', required: true, type: 'password' },
      { key: 'encrypt_key', labelKey: 'fields.encryptKey', type: 'password', group: 'advanced' },
      { key: 'verification_token', labelKey: 'fields.verificationToken', type: 'password', group: 'advanced' },
      { key: 'allow_from', labelKey: 'fields.allowFrom', placeholder: '* (all)', group: 'advanced' },
      { key: 'group_reply_all', labelKey: 'fields.groupReplyAll', type: 'boolean', group: 'advanced' },
      { key: 'share_session_in_channel', labelKey: 'fields.sharedGroupSession', type: 'boolean', group: 'advanced' },
      { key: 'group_context_buffer', labelKey: 'fields.groupContextBuffer', type: 'boolean', group: 'advanced' },
    ],
  },
};
