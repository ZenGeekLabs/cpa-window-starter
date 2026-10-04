const test=require('node:test');
const assert=require('node:assert/strict');
const {mount}=require('../web/timezone-picker.js');
class Target {
 constructor(){this.listeners=new Map();}
 addEventListener(type,fn){if(!this.listeners.has(type))this.listeners.set(type,new Set());this.listeners.get(type).add(fn);}
 removeEventListener(type,fn){this.listeners.get(type)?.delete(fn);}
 dispatchEvent(event){if(!event.target)event.target=this;for(const fn of this.listeners.get(event.type)||[])fn(event);return !event.defaultPrevented;}
}
class Event {
 constructor(type,options={}){this.type=type;Object.assign(this,options);}
 preventDefault(){this.defaultPrevented=true;}
 stopPropagation(){this.propagationStopped=true;}
}
class Element extends Target {
 constructor(doc,tag){super();this.ownerDocument=doc;this.tagName=tag;this.children=[];this.attributes=new Map();this.className='';this.value='';this.readOnly=false;this.disabled=false;this.hidden=false;this.style={setProperty(name,value){this[name]=value;}};
  this.classList={add:name=>{this.className=[...new Set([...this.className.split(' ').filter(Boolean),name])].join(' ');},remove:name=>{this.className=this.className.split(' ').filter(x=>x!==name).join(' ');},toggle:(name,enabled)=>enabled?this.classList.add(name):this.classList.remove(name)};
 }
 append(...children){for(const child of children){child.parentNode=this;this.children.push(child);}}
 replaceChildren(...children){this.children.forEach(child=>child.parentNode=null);this.children=[];this.append(...children);}
 remove(){if(this.parentNode)this.parentNode.children=this.parentNode.children.filter(child=>child!==this);this.parentNode=null;}
 setAttribute(name,value){this.attributes.set(name,String(value));}
 getAttribute(name){return this.attributes.has(name)?this.attributes.get(name):null;}
 removeAttribute(name){this.attributes.delete(name);}
 contains(target){return target===this||this.children.some(child=>child.contains(target));}
 focus(){this.ownerDocument.activeElement=this;}
 getBoundingClientRect(){return {left:20,top:150,bottom:190,width:320};}
 scrollIntoView(){this.scrolled=true;}
}
function environment(){
 const doc=new Target(),win=new Target();win.Event=Event;win.innerWidth=1024;win.innerHeight=768;doc.defaultView=win;doc.createElement=tag=>new Element(doc,tag);doc.body=new Element(doc,'body');
 const input=doc.createElement('input');input.value='Asia/Shanghai';input.setAttribute('list','old-datalist');doc.body.append(input);
 const zones=['America/New_York','Asia/Shanghai','Asia/Tokyo','Europe/London','UTC'];
 const changes=[],callbacks=[];input.addEventListener('input',event=>changes.push({value:input.value,bubbles:event.bubbles}));
 let language='zh-CN';const picker=mount({input,zones,language:()=>language,onChange:value=>callbacks.push(value)});
 const popup=doc.body.children[1],[search,list,status]=popup.children;
 const event=(target,type,props={})=>{const e=new Event(type,props);target.dispatchEvent(e);return e;};
 return {doc,win,input,zones,picker,popup,search,list,status,changes,callbacks,event,setLanguage:value=>{language=value;}};
}
test('click opens every zone and independent search chooses without replacing the input prematurely',()=>{
 const e=environment();assert.equal(e.input.readOnly,true);assert.equal(e.input.getAttribute('list'),null);assert.equal(e.popup.hidden,true);
 e.event(e.input,'click');assert.equal(e.popup.hidden,false);assert.equal(e.input.getAttribute('aria-expanded'),'true');assert.equal(e.search.value,'');assert.equal(e.list.children.length,e.zones.length);assert.equal(e.doc.activeElement,e.search);
 assert.equal(e.list.children.find(option=>option.getAttribute('aria-selected')==='true').children[0].textContent,'Asia/Shanghai');
 e.search.value='new york';e.event(e.search,'input');assert.equal(e.list.children.length,1);assert.equal(e.input.value,'Asia/Shanghai');
 e.event(e.list.children[0],'click');assert.equal(e.input.value,'America/New_York');assert.equal(e.popup.hidden,true);assert.equal(e.doc.activeElement,e.input);
 assert.deepEqual(e.changes,[{value:'America/New_York',bubbles:true}]);assert.deepEqual(e.callbacks,['America/New_York']);
 e.event(e.input,'click');assert.equal(e.search.value,'');assert.equal(e.list.children.length,e.zones.length);
});
test('keyboard selects with arrows and Enter, and Escape or outside interaction closes',()=>{
 const e=environment();assert.equal(e.event(e.input,'keydown',{key:'ArrowDown'}).defaultPrevented,true);
 e.search.value='asia';e.event(e.search,'input');e.event(e.search,'keydown',{key:'ArrowDown'});
 const active=e.search.getAttribute('aria-activedescendant');assert.equal(e.list.children.find(option=>option.id===active).children[0].textContent,'Asia/Tokyo');
 assert.equal(e.event(e.search,'keydown',{key:'Enter'}).defaultPrevented,true);assert.equal(e.input.value,'Asia/Tokyo');assert.equal(e.changes.length,1);
 e.picker.open();e.event(e.search,'keydown',{key:'Escape'});assert.equal(e.popup.hidden,true);assert.equal(e.doc.activeElement,e.input);assert.equal(e.input.getAttribute('aria-activedescendant'),null);
 e.picker.open();e.event(e.doc,'pointerdown',{target:e.doc.body});assert.equal(e.popup.hidden,true);
 e.picker.open();e.event(e.search,'keydown',{key:'Tab'});assert.equal(e.popup.hidden,true);assert.equal(e.doc.activeElement,e.input);
});
test('refresh follows programmatic values and language without producing model or form changes',()=>{
 const e=environment();e.input.value='Pacific/Auckland';e.setLanguage('en');e.picker.refresh();e.picker.open();
 assert.equal(e.search.getAttribute('aria-label'),'Search time zones');assert.equal(e.list.children.length,6);assert.equal(e.status.textContent,'6 time zones');
 assert.equal(e.list.children.find(option=>option.getAttribute('aria-selected')==='true').children[0].textContent,'Pacific/Auckland');
 e.search.value='not-a-zone';e.event(e.search,'input');assert.equal(e.list.children.length,0);assert.equal(e.status.textContent,'No matching time zones');
 e.event(e.search,'keydown',{key:'Enter'});assert.equal(e.input.value,'Pacific/Auckland');assert.equal(e.changes.length,0);
 e.setLanguage('zh-CN');e.picker.refresh();assert.equal(e.status.textContent,'没有匹配的时区');
 e.picker.close();e.picker.open();e.event(e.search,'keydown',{key:'Enter'});assert.equal(e.changes.length,0,'selecting current value is not a form edit');
});
test('disabled trigger cannot open and destroy restores the native input',()=>{
 const e=environment();e.input.disabled=true;e.picker.open();assert.equal(e.popup.hidden,true);e.input.disabled=false;e.picker.open();e.picker.destroy();
 assert.equal(e.doc.body.children.length,1);assert.equal(e.input.readOnly,false);assert.equal(e.input.getAttribute('list'),'old-datalist');assert.equal(e.input.getAttribute('role'),null);assert.equal(e.input.getAttribute('aria-expanded'),null);
 e.event(e.input,'click');assert.equal(e.doc.body.children.length,1);e.picker.refresh();e.picker.destroy();
});
