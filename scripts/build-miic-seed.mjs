// Build local MIIC fixture SQL for the current Dia Portal CMS schema.
// Usage: node scripts/build-miic-seed.mjs <output.sql>
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, resolve } from 'node:path'

const output = process.argv[2]
if (!output) throw new Error('output SQL path is required')

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const adapter = join(root, 'internal', 'adapters', 'miic')
const legacy = readFileSync(join(adapter, 'testdata', 'seed-legacy.sql'), 'utf8').replaceAll('\r', '')
const statement = name => {
  const match = legacy.match(new RegExp('INSERT INTO `' + name + '`[\\s\\S]*?;'))
  if (!match) throw new Error(`missing ${name} seed statement`)
  return match[0]
}

const templates = [
  [1, '资讯动态', 'news', 'home', 'news.html', 'testdata/templates/news.html.tmpl'],
  [2, '核心业务', 'business', 'home', 'business.html', 'site/business.html'],
  [3, '服务平台', 'platforms', 'home', 'platforms.html', 'site/platforms.html'],
  [4, '关于我们', 'about', 'home', 'about.html', 'site/about.html'],
  [5, '栏目', 'list', 'column', 'index.html', 'testdata/templates/list.html.tmpl'],
  [6, '详情', 'article', 'detail', 'article', 'testdata/templates/article.html.tmpl'],
]
const quote = value => "'" + value.replaceAll("'", "''") + "'"
const templateSQL = templates.map(([id, name, code, type, route, file]) => {
  const source = readFileSync(join(adapter, file)).toString('hex')
  return `INSERT INTO template (id,created_at,updated_at,name,code,type,route_path,status,source_code) VALUES (${id},NOW(3),NOW(3),${quote(name)},${quote(code)},${quote(type)},${quote(route)},1,CONVERT(0x${source} USING utf8mb4));`
}).join('\n')

const columns = statement('column').replaceAll('page_id', 'template_id')
  .replace('`status`) VALUES', '`status`,`route_path`) VALUES')
  .split('\n').map(line => {
    const match = line.match(/^  \((\d+),.*\)(,?)$/)
    return match ? line.replace(/\)(,?)$/, `,'list/${match[1]}')$1`) : line
  }).join('\n')
  .replace('`status`=1;', '`status`=1,`route_path`=VALUES(`route_path`);')

const publications = statement('article_column_publish').replaceAll('page_id', 'template_id')
const introduction = readFileSync(join(adapter, 'testdata', 'introduction.html')).toString('hex')
const introductionColumn = "INSERT INTO `column` (id,created_at,updated_at,name,code,template_id,parent_id,route_path,description,sort,status) VALUES (400,NOW(3),NOW(3),'中心介绍','introduction',4,0,'list/400','中心介绍正文',0,1) ON DUPLICATE KEY UPDATE updated_at=VALUES(updated_at),name=VALUES(name),template_id=VALUES(template_id),status=1;"
const introductionArticle = `INSERT INTO article (created_at,updated_at,title,type,summary,content,status,audit_status,publish_time) VALUES (NOW(3),NOW(3),'中心介绍',1,'机械工业信息中心简介。',CONVERT(0x${introduction} USING utf8mb4),1,2,NOW(3)); SET @miic_introduction_article_id = LAST_INSERT_ID();`
const introductionPublication = "INSERT INTO article_column_publish (created_at,updated_at,template_id,column_id,article_id,article_title,is_top) VALUES (NOW(3),NOW(3),4,400,@miic_introduction_article_id,'中心介绍',1);"
const sql = [
  'SET NAMES utf8mb4;',
  'START TRANSACTION;',
  templateSQL,
  statement('category'),
  columns,
  introductionColumn,
  statement('article'),
  introductionArticle,
  publications,
  introductionPublication,
  'COMMIT;',
  '',
].join('\n\n')
writeFileSync(output, sql, 'utf8')
