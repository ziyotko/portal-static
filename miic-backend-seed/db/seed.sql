USE `miic_portal`;

INSERT INTO `category` (`id`,`created_at`,`updated_at`,`name`,`code`,`description`,`sort`,`status`) VALUES
  (1,NOW(3),NOW(3),'默认分类','default','本地静态化联调分类',1,1)
ON DUPLICATE KEY UPDATE `updated_at`=VALUES(`updated_at`),`name`=VALUES(`name`),`status`=1;

INSERT INTO `page` (`id`,`created_at`,`updated_at`,`name`,`code`,`page_type`,`route_path`,`description`,`status`) VALUES
  (1,NOW(3),NOW(3),'资讯动态','news','page','news.html','资讯动态页面',1),
  (2,NOW(3),NOW(3),'核心业务','business','page','business.html','核心业务页面',1),
  (3,NOW(3),NOW(3),'服务平台','platforms','page','platforms.html','服务平台页面',1),
  (4,NOW(3),NOW(3),'关于我们','about','page','about.html','关于我们页面',1)
ON DUPLICATE KEY UPDATE `updated_at`=VALUES(`updated_at`),`name`=VALUES(`name`),`code`=VALUES(`code`),`status`=1;

INSERT INTO `column` (`id`,`created_at`,`updated_at`,`name`,`code`,`page_id`,`parent_id`,`description`,`sort`,`status`) VALUES
  (101,NOW(3),NOW(3),'中心重要动态','important',1,0,'四个主页面共用的重要动态',0,1),
  (102,NOW(3),NOW(3),'最新','latest',1,0,'后台真实栏目',10,1),
  (103,NOW(3),NOW(3),'中心动态','center',1,0,'中心动态',20,1),
  (104,NOW(3),NOW(3),'行业动态','industry',1,0,'行业动态',30,1),
  (105,NOW(3),NOW(3),'行业资讯','information',1,0,'行业资讯',40,1),
  (106,NOW(3),NOW(3),'成果发布','results',1,0,'成果发布',50,1),
  (201,NOW(3),NOW(3),'信息技术服务','it-services',2,0,'信息技术服务',10,1),
  (202,NOW(3),NOW(3),'数智化创新赋能服务','digital-services',2,0,'数智化创新赋能服务',20,1),
  (203,NOW(3),NOW(3),'行业智库咨询服务','consulting-services',2,0,'行业智库咨询服务',30,1),
  (204,NOW(3),NOW(3),'行业公共服务','public-services',2,0,'行业公共服务',40,1),
  (211,NOW(3),NOW(3),'软件产品及定制开发服务','software',2,201,'软件产品及定制开发服务',10,1),
  (212,NOW(3),NOW(3),'系统集成服务','integration',2,201,'系统集成服务',20,1),
  (213,NOW(3),NOW(3),'云资源服务','cloud',2,201,'云资源服务',30,1),
  (214,NOW(3),NOW(3),'信息系统运维服务','operations',2,201,'信息系统运维服务',40,1),
  (215,NOW(3),NOW(3),'安全合规服务','security',2,201,'安全合规服务',50,1),
  (221,NOW(3),NOW(3),'数据要素服务','data-elements',2,202,'数据要素服务',10,1),
  (222,NOW(3),NOW(3),'数智化评价咨询服务','digital-evaluation',2,202,'数智化评价咨询服务',20,1),
  (223,NOW(3),NOW(3),'数智化能力培育服务','digital-training',2,202,'数智化能力培育服务',30,1),
  (231,NOW(3),NOW(3),'战略规划咨询服务','strategy',2,203,'战略规划咨询服务',10,1),
  (232,NOW(3),NOW(3),'产业研究与决策咨询服务','industry-research',2,203,'产业研究与决策咨询服务',20,1),
  (233,NOW(3),NOW(3),'项目评估与论证咨询服务','project-evaluation',2,203,'项目评估与论证咨询服务',30,1),
  (241,NOW(3),NOW(3),'公共信息服务','public-information',2,204,'公共信息服务',10,1),
  (242,NOW(3),NOW(3),'行业交流服务','industry-exchange',2,204,'行业交流服务',20,1),
  (243,NOW(3),NOW(3),'普惠赋能服务','inclusive-empowerment',2,204,'普惠赋能服务',30,1),
  (401,NOW(3),NOW(3),'招聘信息','recruitment',4,0,'招聘岗位',10,1),
  (402,NOW(3),NOW(3),'信息公开','disclosure',4,0,'公开文件与正文',20,1)
ON DUPLICATE KEY UPDATE `updated_at`=VALUES(`updated_at`),`name`=VALUES(`name`),`code`=VALUES(`code`),`page_id`=VALUES(`page_id`),`parent_id`=VALUES(`parent_id`),`sort`=VALUES(`sort`),`status`=1;

INSERT INTO `article` (`id`,`created_at`,`updated_at`,`title`,`type`,`summary`,`content`,`status`,`audit_status`,`is_top`,`cover`,`source`,`url`,`publish_time`) VALUES
  (1001,NOW(3),NOW(3),'聚焦行业数智化转型，构建机械工业高质量发展新动能',1,'围绕数据价值释放、转型能力提升和行业共性需求，持续完善数智化赋能服务体系。','<h2>内容摘要</h2><p>机械工业信息中心持续完善数智化赋能服务体系，为行业高质量发展提供专业支撑。</p>',1,2,0,'assets/unsplash-data-dashboard.jpg','机械工业信息中心','',NOW(3)),
  (1002,NOW(3),NOW(3),'机械工业运行总体平稳，产业结构持续优化升级',1,'跟踪行业运行态势与重点领域变化，为产业分析与科学决策提供专业信息参考。','<h2>行业观察</h2><p>本数据用于验证栏目内置顶、发布时间排序和文章详情生成。</p>',1,2,0,'assets/unsplash-gears.jpg','机械工业信息中心','',DATE_SUB(NOW(3),INTERVAL 1 DAY)),
  (1003,NOW(3),NOW(3),'制造业数字化转型加速向全链条协同纵深推进',1,'从单点技术应用走向流程重构与数据协同。','<h2>行业资讯</h2><p>制造业数字化建设加快从单点应用走向全链条协同。</p>',1,2,0,'assets/unsplash-appliance-line.jpg','机械工业信息中心','',DATE_SUB(NOW(3),INTERVAL 2 DAY)),
  (1004,NOW(3),NOW(3),'机械工业数字化转型路径与实践研究成果发布',1,'总结典型实践，提炼转型方法。','<h2>成果摘要</h2><p>形成面向行业企业的可复用数字化转型经验。</p>',1,2,0,'assets/unsplash-precision-lab.jpg','机械工业信息中心','',DATE_SUB(NOW(3),INTERVAL 3 DAY)),
  (2001,NOW(3),NOW(3),'软件产品及定制开发服务',1,'面向机关单位与行业协会提供标准化软件产品、定制开发及实施服务。','<h2>服务内容</h2><p>提供门户网站、协同办公、会员管理、会议管理等标准产品，并支持按业务需求开展定制开发。</p><h2>服务流程</h2><ol><li>需求沟通</li><li>方案设计</li><li>开发实施</li><li>上线运维</li></ol>',1,2,0,'assets/unsplash-software-development.jpg','机械工业信息中心','',NOW(3)),
  (3001,NOW(3),NOW(3),'前端工程师',1,'负责门户及业务系统前端开发，招聘状态以后台发布信息为准。','<h2>岗位职责</h2><ul><li>负责门户及业务系统前端功能开发。</li><li>配合完成页面性能和兼容性优化。</li></ul><h2>应聘方式</h2><p>请将简历发送至 zhaopin@miic.com.cn。</p>',1,2,0,'','机械工业信息中心','',NOW(3)),
  (3002,NOW(3),NOW(3),'2026年机械工业信息中心部门预算',1,'2026年度部门预算公开文件。','',1,2,0,'','机械工业信息中心','https://www.miic.com.cn/api/v1/file/info?filePath=inforurl/202604/01f84cf78400200e.pdf&fileName=2026年机械工业信息中心部门预算.pdf',NOW(3))
ON DUPLICATE KEY UPDATE `updated_at`=VALUES(`updated_at`),`title`=VALUES(`title`),`summary`=VALUES(`summary`),`content`=VALUES(`content`),`status`=1,`audit_status`=2,`cover`=VALUES(`cover`),`source`=VALUES(`source`),`url`=VALUES(`url`),`publish_time`=VALUES(`publish_time`);

INSERT INTO `article_column_publish` (`id`,`created_at`,`updated_at`,`page_id`,`column_id`,`article_id`,`article_title`,`is_top`) VALUES
  (10001,NOW(3),NOW(3),1,101,1001,'聚焦行业数智化转型，构建机械工业高质量发展新动能',1),
  (10002,NOW(3),NOW(3),1,101,1002,'机械工业运行总体平稳，产业结构持续优化升级',0),
  (10003,NOW(3),NOW(3),1,102,1001,'聚焦行业数智化转型，构建机械工业高质量发展新动能',1),
  (10004,NOW(3),NOW(3),1,102,1002,'机械工业运行总体平稳，产业结构持续优化升级',0),
  (10005,NOW(3),NOW(3),1,103,1001,'聚焦行业数智化转型，构建机械工业高质量发展新动能',1),
  (10006,NOW(3),NOW(3),1,104,1002,'机械工业运行总体平稳，产业结构持续优化升级',1),
  (10007,NOW(3),NOW(3),1,105,1003,'制造业数字化转型加速向全链条协同纵深推进',1),
  (10008,NOW(3),NOW(3),1,106,1004,'机械工业数字化转型路径与实践研究成果发布',1),
  (20001,NOW(3),NOW(3),2,211,2001,'软件产品及定制开发服务',1),
  (30001,NOW(3),NOW(3),4,401,3001,'前端工程师',1),
  (30002,NOW(3),NOW(3),4,402,3002,'2026年机械工业信息中心部门预算',1)
ON DUPLICATE KEY UPDATE `updated_at`=VALUES(`updated_at`),`page_id`=VALUES(`page_id`),`article_title`=VALUES(`article_title`),`is_top`=VALUES(`is_top`);
