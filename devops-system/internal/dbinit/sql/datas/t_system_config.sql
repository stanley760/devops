-- t_system_config initial data: tenant 1 (default) system configurations.
-- type=1 built-in items (editable but not deletable), type=2 custom items.
-- Explicit ID + tenant_id. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_config
    (id, tenant_id, creator, create_time, updater, update_time,
     config_key, config_name, config_value, type, visible, remark)
VALUES
    (1, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.account.registerUser', '是否允许用户注册', 'false', 1, TRUE,  '内置配置项，不可删除'),
    (2, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.account.captchaEnabled', '是否开启验证码', 'true', 1, TRUE,  '内置配置项，不可删除'),
    (3, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.account.defaultPassword', '用户初始密码', '123456', 1, FALSE, '内置配置项，不可删除'),
    (4, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.ui.title', '控制台标题', 'DevOps 管理平台', 1, TRUE, '内置配置项，不可删除'),
    (5, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.ui.logo', '控制台 Logo', '', 1, TRUE, '内置配置项，不可删除'),
    (6, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.ui.theme', '控制台主题', 'dark', 1, TRUE, '内置配置项，不可删除'),
    -- Captcha: login / register verification.
    -- sys.captcha.type must be one of the sys_captcha_type dictionary values (click / slide / rotate);
    -- an out-of-dictionary value falls back to click. Read per request, so changes take effect immediately.
    (7, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.captcha.enabled', '是否开启验证码', 'true', 1, FALSE, '内置配置项，不可删除；登录/注册验证码总开关'),
    (8, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys.captcha.type', '验证码类型', 'click', 1, FALSE, '内置配置项，不可删除；取值范围见字典 sys_captcha_type：click/slide/dragdrop/rotate')
ON DUPLICATE KEY UPDATE id = id;
