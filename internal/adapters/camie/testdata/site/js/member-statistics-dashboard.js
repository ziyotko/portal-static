/**
 * 环保装备数据大屏 - 无框架、无外部依赖、纯 SVG 图表实现
 * 视觉样式来自用户提供的环保装备数据大屏源码；统计值只由会员接口提供。
 */
(function (global) {
  'use strict';

  function esc(value) { return String(value ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch])); }
  function fmt(value, digits=0) { return Number(value).toLocaleString('zh-CN',{minimumFractionDigits:digits,maximumFractionDigits:digits}); }
  function whole(value) { return fmt(Math.trunc(Number(value) || 0)); }
  function pts(n){return Math.round(n*10)/10;}
  const g = (icon) => `<svg viewBox="0 0 48 48" class="env-kpi-icon" fill="none" aria-hidden="true">${icon}</svg>`;
  const icons = {
    city: g('<path d="M6 40V13l14-5v32M23 40V5l16 6v29M3 40h42" stroke="currentColor" stroke-width="3" stroke-linejoin="round"/><path d="M12 20h3m-3 7h3m-3 7h3m15-19h3m-3 7h3m-3 7h3" stroke="currentColor" stroke-width="3"/>'),
    people: g('<circle cx="24" cy="14" r="8" fill="currentColor"/><circle cx="9" cy="20" r="6" fill="currentColor"/><circle cx="39" cy="20" r="6" fill="currentColor"/><path d="M12 42V33c0-8 5-11 12-11s12 3 12 11v9zM2 41V31c0-5 3-8 8-8M46 41V31c0-5-3-8-8-8" fill="currentColor"/>'),
    coins: g('<ellipse cx="23" cy="9" rx="14" ry="5" fill="currentColor"/><path d="M9 10v7c0 7 28 7 28 0v-7M9 21v7c0 7 28 7 28 0v-7M9 32v7c0 7 28 7 28 0v-7" stroke="currentColor" stroke-width="3"/><path d="M9 17c0 7 28 7 28 0M9 28c0 7 28 7 28 0" stroke="currentColor" stroke-width="2"/>'),
    rise: g('<path d="M4 40h40M9 36V25m9 11V18m9 18V24m9 12V12" stroke="currentColor" stroke-width="5" stroke-linecap="round"/><path d="M5 22 19 9l11 9L44 4m0 0H32m12 0v12" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>')
  };
  const decor = {
    city: '<path d="M0 70h110V25H83V0H64v40H43V22H20v48z" fill="currentColor"/>',
    bars: '<path d="M3 70V42h18v28zm30 0V23h18v47zm30 0V8h18v62z" fill="currentColor"/>',
    rise: '<path d="M5 55 35 29l24 15L95 3M79 3h16v16" fill="none" stroke="currentColor" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>',
    dots: '<path d="M10 52 39 23l19 16L86 12" fill="none" stroke="currentColor" stroke-width="5"/><circle cx="10" cy="52" r="5" fill="currentColor"/><circle cx="39" cy="23" r="5" fill="currentColor"/><circle cx="58" cy="39" r="5" fill="currentColor"/><circle cx="86" cy="12" r="5" fill="currentColor"/>'
  };
  function kpi(label,value,unit,iconName,isOrange=false){
    return `<article class="env-kpi"><div class="env-kpi-graphic${isOrange?' orange':''}">${icons[iconName]}</div><div><div class="env-kpi-label">${esc(label)}</div><div class="env-kpi-value${isOrange?' orange':''}">${esc(value)} <small>${esc(unit)}</small></div></div><svg class="env-kpi-decoration" viewBox="0 0 110 75" aria-hidden="true">${decor[isOrange?'dots':iconName==='coins'?'rise':iconName==='people'?'bars':'city']}</svg></article>`;
  }
  const legend = `<div class="env-chart-legend"><span><i class="env-legend-swatch"></i>主营业务收入（亿元）</span><span><i class="env-legend-dot"></i>同比增速（%）</span></div>`;
  function panelTitle(title){return `<div class="env-panel-title">${esc(title)}</div>`;}
  function filterOptions(names){return '<option value="">全部</option>'+(names || []).map(name=>`<option value="${esc(name)}">${esc(name)}</option>`).join('');}
  function yearOptions(data){return (data.years || [data.year]).map(year=>`<option value="${esc(year)}" ${String(year)===String(data.year)?'selected':''}>${esc(year)}年</option>`).join('');}
  function regionOptions(data){return ['',...(data.filters.regions || [])].map((name,i)=>`<label class="env-radio"><input type="radio" name="env-region" value="${esc(name)}" ${i===0?'checked':''}><span>${esc(name || '全国')}</span></label>`).join('');}
  function shell(data){
    return `<div class="env-dashboard">
      <header class="env-header">
        <h1 class="env-hero-title"><svg class="env-leaf" viewBox="0 0 40 44" aria-hidden="true"><path d="M23 2C5 13 5 27 17 36 31 32 38 19 23 2Z" fill="#4cb46f"/><path d="M11 39c8-15 14-19 23-28" stroke="#e4ffeb" stroke-width="3" fill="none" stroke-linecap="round"/></svg>环保装备制造业经济运行情况统计信息（<span data-year-title>${esc(data.year)}</span>年）</h1>
        <div class="env-tagline">数 据 赋 能 行 业 发 展　　绿 色 装 备 助 力 美 丽 中 国</div>
        <span class="env-update">数据更新时间：${esc(data.updateDate)}</span>
        <button type="button" class="env-info-btn" data-info><span aria-hidden="true">ⓘ</span> 数据说明</button>
      </header>
      <div class="env-layout">
        <aside class="env-sidebar" aria-label="数据筛选">
          <div class="env-filter-header">⚑ &nbsp;数据筛选 <button type="button" class="env-reset" data-reset aria-label="重置筛选">重置</button></div>
          <div class="env-filter-inner">
            <section class="env-filter-block"><h2 class="env-filter-title">年份</h2><select class="env-year" data-year aria-label="年份">${yearOptions(data)}</select></section>
            <section class="env-filter-block"><h2 class="env-filter-title">地区范围</h2><div class="env-radio-list" data-region-list>
              ${regionOptions(data)}
            </div></section>
            <section class="env-filter-block"><h2 class="env-filter-title">省份</h2><select class="env-year" data-province aria-label="省份">${filterOptions(data.filters.provinces)}</select></section>
            <section class="env-filter-block"><h2 class="env-filter-title">细分领域</h2><select class="env-year" data-sub-field aria-label="细分领域">${filterOptions(data.filters.subFields)}</select></section>
            <section class="env-filter-block env-flags"><h2 class="env-filter-title">一带 / 一轴</h2><label>一带 <select data-is-belt aria-label="一带"><option value="">全部</option><option value="1">是</option><option value="0">否</option></select></label><label>一轴 <select data-is-axis aria-label="一轴"><option value="">全部</option><option value="1">是</option><option value="0">否</option></select></label></section>
          </div>
        </aside>
        <main class="env-main">
          <section class="env-kpis" data-kpis aria-label="核心指标"></section>
          <section class="env-chart-panel env-big-chart"><div class="env-panel-head">${panelTitle('各省市主营业务收入及同比增速统计信息')}${legend}</div><div class="env-svg-holder" data-chart="provinces"></div></section>
          <section class="env-bottom">
            <div class="env-chart-panel env-small-chart"><div class="env-panel-head">${panelTitle('各省市企业数量统计信息（家）')}</div><div class="env-svg-holder" data-chart="companies"></div></div>
            <div class="env-chart-panel env-small-chart"><div class="env-panel-head">${panelTitle('细分领域主营业务收入统计信息（亿元）')}</div><div class="env-svg-holder" data-chart="segments"></div></div>
            <div class="env-chart-panel env-small-chart"><div class="env-panel-head">${panelTitle('各地区企业主营业务收入及同比增速统计信息')}</div><div class="env-svg-holder"><div class="env-panel-legend-floating"><span><i class="env-legend-swatch"></i>主营业务收入（亿元）</span><span><i class="env-legend-dot"></i>同比增速（%）</span></div><div data-chart="regions" style="height:100%"></div></div></div>
          </section>
          <div class="env-footnote">提示：图表为 SVG 绘制，在移动设备上可横向查看上方趋势图。</div>
        </main>
      </div>
      <div class="env-tooltip" role="tooltip"></div>
      <div class="env-modal-bg" data-modal><div class="env-modal" role="dialog" aria-modal="true" aria-label="数据说明"><h2>数据说明</h2><p>展示已发布的 ${esc(data.year)} 年会员统计数据，对比年份为 ${esc(data.comparisonYear)} 年。数据导入日期：${esc(data.updateDate)}。</p><p>收入按原表万元汇总后换算为亿元；同比按当前筛选范围内两年收入合计计算。当前范围包含 <span data-record-count>${esc(data.recordCount)}</span> 条原始记录，企业名称去重后为顶部样本企业数。</p><p>地区、省份、细分领域及一带、一轴筛选会重新请求后台汇总；省份和细分领域依照当前接口支持单选。</p><button type="button" data-close-modal>知道了</button><div style="clear:both"></div></div></div>
    </div>`;
  }

  const text = (x,y,v,anchor='middle',cls='env-svg-text',extra='')=>`<text x="${pts(x)}" y="${pts(y)}" text-anchor="${anchor}" class="${cls}" ${extra}>${esc(v)}</text>`;
  const barGradient = id => `<defs><linearGradient id="${id}" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#80e3b7"/><stop offset="1" stop-color="#48ca8e"/></linearGradient></defs>`;
  const svg = (w,h,content)=>`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none" role="img">${content}</svg>`;
  const tooltipAttr = str => `data-tip="${esc(str)}"`;

  function mainChart(items){
    const W=1320,H=230,L=69,R=44,T=20,B=192;
    const PW=W-L-R, PH=B-T, step=PW/Math.max(items.length,1), barW=Math.min(23,step*.52);
    const key='main-mint';
    let out=barGradient(key);
    for(let tick=0;tick<=1000;tick+=200){let y=B-tick/1000*PH;out+=`<line x1="${L}" y1="${pts(y)}" x2="${W-R}" y2="${pts(y)}" class="env-svg-grid"/>`;out+=text(L-12,y+4,tick,'end');}
    for(let tick=-40;tick<=40;tick+=20){const y=B-((tick+40)/80)*PH;out+=text(W-R+17,y+4,tick+'%','start');}
    out+=`<line x1="${L}" y1="${B}" x2="${W-R}" y2="${B}" class="env-svg-axis"/>`;
    items.forEach((d,i)=>{
      const x=L+i*step+step/2;
      const barH=Math.max(1,d.revenue/1000*PH),by=B-barH;
      out+=`<rect x="${pts(x-barW/2)}" y="${pts(by)}" width="${pts(barW)}" height="${pts(barH)}" fill="url(#${key})" ${tooltipAttr(`${d.name}｜主营业务收入：${d.revenue} 亿元${d.growth!=null?'｜同比增速：'+d.growth+'%':''}`)}/>`;
      out+=text(x,Math.max(T+8,by-7),whole(d.revenue),'middle','env-svg-number');
      out+=text(x,B+20,d.name,'middle','env-svg-text','font-size="11"');
      if(d.growth!=null){const gy=B-((d.growth+40)/80)*PH;
        out+=`<circle cx="${pts(x)}" cy="${pts(gy)}" r="3.6" fill="#fa7b19" stroke="#fff" stroke-width="1" ${tooltipAttr(`${d.name}｜同比增速：${d.growth}%`)}/>`;
        if(items.length <= 15 || i<12 || i%2===0){
          const ly=gy < 38 ? gy+17 : gy-9;
          out+=text(x+5,ly,`${whole(d.growth)}%`,'start','env-svg-growth');
        }
      }
    });
    if(items.length===0){out+=text(W/2,100,'当前筛选无可展示的省市数据');}
    return svg(W,H,out);
  }

  function companiesChart(items){
    const W=480,H=312,L=55,R=27,T=23,B=282, PW=W-L-R,PH=B-T;
    let out=barGradient('count-mint');
    for(let tick=0;tick<=150;tick+=30){let x=L+PW*tick/150;out+=`<line x1="${pts(x)}" y1="${T}" x2="${pts(x)}" y2="${B}" class="env-svg-grid"/>`;out+=text(x,300,tick);}
    const cnt=Math.max(1,items.length), step=Math.min(15,PH/cnt),barH=Math.max(4,step*.65);
    items.forEach((d,i)=>{
      const y=T+i*step, bw=Math.max(1,Math.min(150,d.count)/150*PW);
      out+=text(L-9,y+9,d.name,'end','env-svg-text');
      out+=`<rect x="${L}" y="${pts(y+2)}" width="${pts(bw)}" height="${pts(barH)}" fill="url(#count-mint)" ${tooltipAttr(`${d.name}：${d.count} 家企业`)}/>`;
      out+=text(Math.min(W-20,L+bw+6),y+9,d.count,'start','env-svg-number');
    });
    if(items.length===0){out+=text(W/2,H/2,'当前筛选无企业数量数据');}
    return svg(W,H,out);
  }

  function segmentsChart(items,selected){
    const W=468,H=312,L=54,R=10,T=35,B=244,PW=W-L-R,PH=B-T;
    let out=barGradient('segment-mint');
    for(let tick=0;tick<=1800;tick+=300){const y=B-tick/1800*PH;out+=`<line x1="${L}" y1="${pts(y)}" x2="${W-R}" y2="${pts(y)}" class="env-svg-grid"/>`;out+=text(L-9,y+4,tick,'end');}
    const step=PW/items.length,barW=Math.min(25,step*.6);
    items.forEach((d,i)=>{
      const x=L+(i+.5)*step,barH=Math.max(d.revenue>0?1:0,d.revenue/1800*PH),y=B-barH;
      const opacity=selected.size&&!selected.has(d.name)?'.18':'1';
      out+=`<rect x="${pts(x-barW/2)}" y="${pts(y)}" width="${pts(barW)}" height="${pts(barH)}" opacity="${opacity}" fill="url(#segment-mint)" ${tooltipAttr(`${d.name}：${d.revenue} 亿元`)}/>`;
      out+=text(x,Math.max(T+7,y-8),whole(d.revenue),'middle','env-svg-number',`opacity="${opacity}"`);
      out+=`<text x="${pts(x+7)}" y="${B+22}" text-anchor="end" class="env-svg-text" transform="rotate(-44 ${pts(x+7)} ${B+22})" opacity="${opacity}">${esc(d.name)}</text>`;
    });
    return svg(W,H,out);
  }

  function regionsChart(items){
    const W=495,H=312,L=55,R=32,T=50,B=247,PW=W-L-R,PH=B-T, step=PW/items.length,barW=Math.min(29,step*.48);
    let out=barGradient('region-mint');
    for(let tick=0;tick<=1800;tick+=300){const y=B-tick/1800*PH;out+=`<line x1="${L}" y1="${pts(y)}" x2="${W-R}" y2="${pts(y)}" class="env-svg-grid"/>`;out+=text(L-10,y+4,tick,'end');}
    for(let tick=-20;tick<=20;tick+=10){const y=B-(tick+20)/40*PH;out+=text(W-R+8,y+4,tick+'%','start');}
    items.forEach((d,i)=>{
      const x=L+(i+.5)*step,barH=d.revenue/1800*PH,y=B-barH;
      out+=`<rect x="${pts(x-barW/2)}" y="${pts(y)}" width="${barW}" height="${pts(barH)}" fill="url(#region-mint)" ${tooltipAttr(`${d.name}｜主营业务收入：${d.revenue} 亿元${d.growth!=null?'｜同比增速：'+d.growth+'%':''}`)}/>`;
      out+=text(x,Math.max(T-1,y-9),whole(d.revenue),'middle','env-svg-number');
      out+=text(x,B+22,d.name,'middle','env-svg-text');
      if(d.growth!=null){const gy=B-((d.growth+20)/40)*PH;
        out+=`<circle cx="${pts(x)}" cy="${pts(gy)}" r="4.6" fill="#fc7b14" stroke="white" stroke-width="1" ${tooltipAttr(`${d.name}｜同比增速：${d.growth}%`)}/>`;
        const above=d.growth>5 || d.growth < -7;
        out+=text(x+10,gy+(above?-7:4),`${whole(d.growth)}%`,'start','env-svg-growth');
      }
    });
    return svg(W,H,out);
  }

  function mountEnvironmentDashboard(target, initialData, options = {}) {
    if(!target) throw new Error('mountEnvironmentDashboard: target element is required');
    if(!initialData) throw new Error('mountEnvironmentDashboard: published statistics are required');
    const data = {...initialData};
    const state={ year:String(data.year), region:'', province:'', subField:'', isBelt:'', isAxis:'' };
    target.innerHTML=shell(data);
    const root=target.querySelector('.env-dashboard'), tooltip=root.querySelector('.env-tooltip');
    const disposers=[];
    const on=(el,evt,fn)=>{el.addEventListener(evt,fn);disposers.push(()=>el.removeEventListener(evt,fn));};
    const qs=selector=>root.querySelector(selector);
    const all=selector=>Array.from(root.querySelectorAll(selector));
    function update(){
      qs('[data-kpis]').innerHTML=
        kpi('覆盖省份',fmt(data.headline.provinceCount),'个','city')+
        kpi('样本企业数',fmt(data.headline.companyCount),'家','people')+
        kpi('主营业务收入',fmt(data.headline.revenue,1),'亿元','coins')+
        kpi('同比增速',data.headline.yoy==null?'—':fmt(data.headline.yoy,1)+'%','','rise',true);
      qs('[data-chart="provinces"]').innerHTML=mainChart(data.provinces);
      qs('[data-chart="companies"]').innerHTML=companiesChart(data.companies);
      qs('[data-chart="segments"]').innerHTML=segmentsChart(data.segments,new Set());
      qs('[data-chart="regions"]').innerHTML=regionsChart(data.regions);
    }
    function sync(){
      qs('[data-year]').value=state.year;
      qs('[data-province]').value=state.province;
      qs('[data-sub-field]').value=state.subField;
      all('[name="env-region"]').forEach(i=>i.checked=state.region===i.value);
      qs('[data-is-belt]').value=state.isBelt;
      qs('[data-is-axis]').value=state.isAxis;
    }
    const emit=()=>{if(typeof options.onFilter==='function') options.onFilter({year:state.year,region:state.region,province:state.province,subField:state.subField,isBelt:state.isBelt,isAxis:state.isAxis});};
    on(root,'change',e=>{
      const t=e.target;
      if(t.matches('[data-year]')){state.year=t.value; state.region=state.province=state.subField=state.isBelt=state.isAxis='';}
      else if(t.matches('[name="env-region"]')){state.region=t.value;state.province='';}
      else if(t.matches('[data-province]')){state.province=t.value;}
      else if(t.matches('[data-sub-field]')){state.subField=t.value;}
      else if(t.matches('[data-is-belt]')){state.isBelt=t.value;}
      else if(t.matches('[data-is-axis]')){state.isAxis=t.value;}
      else return;
      sync();emit();
    });
    on(qs('[data-reset]'),'click',()=>{state.region=state.province=state.subField=state.isBelt=state.isAxis='';sync();emit();});
    on(qs('[data-info]'),'click',()=>{qs('[data-modal]').classList.add('is-open');});
    on(qs('[data-close-modal]'),'click',()=>{qs('[data-modal]').classList.remove('is-open');});
    on(qs('[data-modal]'),'click',e=>{if(e.target===qs('[data-modal]'))qs('[data-modal]').classList.remove('is-open');});
    on(root,'pointermove',e=>{
      const hit=e.target.closest('[data-tip]');
      if(!hit){tooltip.style.display='none';return;}
      tooltip.textContent=hit.getAttribute('data-tip');
      tooltip.style.display='block';
      tooltip.style.left=Math.min(e.clientX+15,window.innerWidth-260)+'px';
      tooltip.style.top=Math.min(e.clientY+14,window.innerHeight-74)+'px';
    });
    on(root,'pointerleave',()=>{tooltip.style.display='none';});
    update();
    return {
      destroy(){disposers.splice(0).forEach(fn=>fn());target.innerHTML='';},
      reset(){qs('[data-reset]').click();},
      setData(next){
        Object.assign(data,next);
        state.year=String(data.year);
        qs('[data-year-title]').textContent=data.year;
        qs('.env-update').textContent='数据更新时间：'+data.updateDate;
        qs('[data-year]').innerHTML=yearOptions(data);
        qs('[data-region-list]').innerHTML=regionOptions(data);
        qs('[data-province]').innerHTML=filterOptions(data.filters.provinces);
        qs('[data-sub-field]').innerHTML=filterOptions(data.filters.subFields);
        qs('[data-record-count]').textContent=data.recordCount;
        qs('[data-modal] p').textContent=`展示已发布的 ${data.year} 年会员统计数据，对比年份为 ${data.comparisonYear} 年。数据导入日期：${data.updateDate}。`;
        sync();update();
      }
    };
  }
  global.mountEnvironmentDashboard=mountEnvironmentDashboard;
})(window);
