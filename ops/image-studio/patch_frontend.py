"""Small, version-checked native UI hooks for the pinned Open WebUI build."""
import shutil
from pathlib import Path


def replace_once(source, before, after):
    if source.count(before) != 1:
        raise RuntimeError(f"Open WebUI v0.11.4 frontend anchor changed: {before[:80]}")
    return source.replace(before, after, 1)


def patch_model_selector(source):
    source = replace_once(
        source,
        'import"./CWj6FrbW.js";',
        'import {GroupPicker as NewApiGroupPicker} from "/static/studio-group-picker.js?v=23";'
        'import {W as NewApiBranch} from "./DtFjAPax.js";'
        'import"./CWj6FrbW.js";',
    )
    source = replace_once(
        source,
        'var Ee=fe=>{var A=Eo(),W=d(A);',
        'var Ee=fe=>{NewApiBranch(()=>NewApiGroupPicker(fe,{}));var A=Eo(),W=d(A);',
    )
    return replace_once(
        source,
        'ce("click",je,()=>{p(j,!1),Na.set("admin:connections")}),h(pe,te)',
        'He.hidden=true,je.hidden=true,He.style.display="none",je.style.display="none",h(pe,te)',
    )


def patch_chat_preferences(source):
    source = replace_once(
        source, 'var um=Object.defineProperty;',
        'import {saveChatModels as NewApiSaveChatModels} from "/static/studio-image-state.js?v=23";'
        'var um=Object.defineProperty;',
    )
    return replace_once(
        source, 'get values(){return g()},set values(P){g(P)}',
        'get values(){return g()},set values(P){g(P);'
        'NewApiSaveChatModels(P).catch(()=>_t.error(s().t("Failed to save model preferences")))}',
    )


def patch_chat(source):
    source = replace_once(
        source, 'import"./CWj6FrbW.js";',
        'import {ImagePicker as NewApiImagePicker,imageModels as NewApiImageModels,'
        'imageSelectionError as NewApiImageError} from "/static/studio-image-picker.js?v=23";'
        'import {ImageSizePicker as NewApiImageSizePicker} from "/static/studio-image-size-picker.js?v=23";'
        'import {imagePreferences as NewApiImagePreferences,saveImagePreference as NewApiSaveImagePreference} '
        'from "/static/studio-image-state.js?v=23";'
        'import {W as NewApiBranch} from "./DtFjAPax.js";import"./CWj6FrbW.js";',
    )
    # Bind through Chat -> landing input -> MessageInput, including native draft saving.
    for before, after in [
        ('tt=p(t,"imageGenerationEnabled",12,!1)',
         'newApiImageSize=p(t,"imageGenerationSize",12,"auto"),newApiImageModel=p(t,"imageGenerationModel",12,null),tt=p(t,"imageGenerationEnabled",12,!0)'),
        ('B=p(t,"imageGenerationEnabled",12,!1),M=p(t,"codeInterpreterEnabled"',
         'newApiImageSize=p(t,"imageGenerationSize",12,"auto"),newApiImageModel=p(t,"imageGenerationModel",12,null),B=p(t,"imageGenerationEnabled",12,!0),M=p(t,"codeInterpreterEnabled"'),
        ('Kt=I(!1),Ot=I(!1),Wt=I(!1)',
         'newApiImagePreference=NewApiImagePreferences(n()),newApiImageSize=I(newApiImagePreference.size),'
         'newApiImageModel=I(newApiImagePreference.model),Kt=I(!0),Ot=I(!1),Wt=I(!1)'),
        ('D(Je()),D(tt()),D(Xe())', 'D(Je()),D(newApiImageSize()),D(newApiImageModel()),D(tt()),D(Xe())'),
        ('imageGenerationEnabled:tt(),', 'imageGenerationSize:newApiImageSize(),imageGenerationModel:newApiImageModel(),imageGenerationEnabled:tt(),'),
        ('imageGenerationEnabled:r(Kt),', 'imageGenerationSize:r(newApiImageSize),imageGenerationModel:r(newApiImageModel),imageGenerationEnabled:r(Kt),'),
        ('c(Kt,q.imageGenerationEnabled??!1)',
         '"imageGenerationSize" in q&&c(newApiImageSize,q.imageGenerationSize),'
         '"imageGenerationModel" in q&&c(newApiImageModel,q.imageGenerationModel),c(Kt,q.imageGenerationEnabled??!0)'),
        ('get imageGenerationEnabled(){return B()}',
         'get imageGenerationSize(){return newApiImageSize()},set imageGenerationSize(value){newApiImageSize(value)},get imageGenerationModel(){return newApiImageModel()},set imageGenerationModel(value){newApiImageModel(value)},get imageGenerationEnabled(){return B()}'),
        ('let w={};return(C=o())!=null&&C.features&&(w={voice:W(),',
         'let w={};return(C=o())!=null&&C.features&&(w={image_generation_size:r(newApiImageSize),image_generation_model:r(newApiImageModel),voice:W(),'),
        ('c(Kt,rs.info.meta.defaultFeatureIds.includes("image_generation"))', 'r(Kt)'),
        ('?r(Kt):!1,code_interpreter:',
         '?r(Kt)&&NewApiImageModels().some(model=>model.id===r(newApiImageModel)):!1,code_interpreter:'),
        ('const Rt=()=>i().filter(w=>',
         'Zs(()=>{const reset=()=>{c(ye,[""]);sessionStorage.removeItem("selectedModels")};'
         'window.addEventListener("new-api-group-changed",reset);'
         'return()=>window.removeEventListener("new-api-group-changed",reset)});'
         'const Rt=()=>i().filter(w=>'),
    ]:
        source = replace_once(source, before, after)
    # Both controls must belong to the if-block's DOM range, not its outer anchor.
    start = source.index('var co=w=>')
    end = source.index(';se(Ki,', start)
    block = source[start:end]
    block = replace_once(
        block, 'var co=w=>{{let C=',
        'var co=w=>{var newApiRow=O(\'<span class="new-api-image-controls"><!> <!> <!></span>\')(),'
        'newApiToggle=v(newApiRow);{let C=',
    )
    block = replace_once(block, 'Dt(w,{', 'Dt(newApiToggle,{')
    block = replace_once(
        block, 'b(q,Ee)},$$slots:{default:!0}})}}',
         'b(q,Ee)},$$slots:{default:!0}})}'
         'NewApiBranch(()=>NewApiImagePicker(E(newApiToggle,2),{get value(){return newApiImageModel()},'
         'onChange:value=>newApiImageModel(value),onSelect:value=>{newApiImageModel(value);'
         'NewApiSaveImagePreference("model",value).catch(()=>_t.error(n().t("Failed to save image preferences")))}}));'
         'NewApiBranch(()=>NewApiImageSizePicker(E(newApiToggle,4),{get model(){return newApiImageModel()},'
         'get value(){return newApiImageSize()},onChange:value=>{newApiImageSize(value);'
         'NewApiSaveImagePreference("size",value).catch(()=>_t.error(n().t("Failed to save image preferences")))}}));'
         'f(newApiRow);b(w,newApiRow)}',
    )
    source = source[:start] + block + source[end:]
    for binding in ('Nt', 'Ft', 'mt'):
        source = replace_once(
            source, f'get imageGenerationEnabled(){{return r(Kt)}},set imageGenerationEnabled({binding})',
            'get imageGenerationSize(){return r(newApiImageSize)},'
            'set imageGenerationSize(value){c(newApiImageSize,value)},'
            'get imageGenerationModel(){return r(newApiImageModel)},'
            'set imageGenerationModel(value){c(newApiImageModel,value)},'
            f'get imageGenerationEnabled(){{return r(Kt)}},set imageGenerationEnabled({binding})',
        )
    # Model changes keep the user's toggle; navigation starts from per-user preferences.
    source = replace_once(source, 'c(ur,[]),c(Ot,!1),c(Kt,!1),c(Wt,!1)', 'c(ur,[]),c(Ot,!1),c(Wt,!1)')
    if source.count('c(Kt,!1)') != 3:
        raise RuntimeError("Open WebUI image draft reset anchors changed")
    source = source.replace(
        'c(Kt,!1)',
        'c(Kt,!0),newApiImagePreference=NewApiImagePreferences(n()),'
        'c(newApiImageSize,newApiImagePreference.size),c(newApiImageModel,newApiImagePreference.model)',
    )
    return patch_chat_preferences(source)


def patch_image_settings(source):
    source = replace_once(
        source, 'import"../chunks/CWj6FrbW.js";',
        'import {ImageGroupSetting as NewApiImageGroupSetting} from "/static/studio-image-settings.js?v=23";'
        'import {W as NewApiBranch} from "../chunks/DtFjAPax.js";import"../chunks/CWj6FrbW.js";',
    )
    source = replace_once(
        source, 'if(e(v).IMAGE_GENERATION_ENGINE==="openai"&&e(v).IMAGES_OPENAI_API_KEY==="")',
        'if(!e(v).NEW_API_BRIDGE_ENABLED&&e(v).IMAGE_GENERATION_ENGINE==="openai"&&e(v).IMAGES_OPENAI_API_KEY==="")',
    )
    source = replace_once(
        source, 'return Ae?(ur.set(await zl()),Ae.ENABLE_IMAGE_GENERATION&&Ce(),Ae):null',
        'return Ae?(ur.set(await zl()),window.dispatchEvent(new Event("new-api-image-config-changed")),'
        'Ae.ENABLE_IMAGE_GENERATION&&Ce(),Ae):null',
    )
    # Reuse the existing model field's slot, within the native generation block.
    return replace_once(
        source, 'var W=F=>{var P=RT(),k=Ze(P),V=r(k);{let ee=',
        'var W=F=>{var P=RT(),k=Ze(P),V=r(k);'
        'if(e(v).NEW_API_BRIDGE_ENABLED)NewApiBranch(()=>NewApiImageGroupSetting(V,{'
        'get value(){return e(v).NEW_API_IMAGE_GROUP},onChange:value=>He(v,e(v).NEW_API_IMAGE_GROUP=value)}));'
        'else{let ee=',
    )


def patch_tool_results(source):
    source = replace_once(
        source,
        'const $=new Set([',
        'import {formatToolArguments as NewApiToolArguments,formatToolOutput as NewApiToolOutput} from "'
        '/static/studio-tool-result.js?v=23";const $=new Set([',
    )
    source = replace_once(
        source,
        'text:O(s),attributes:{type:"tool_calls",',
        'text:NewApiToolOutput(n.name,O(s)),attributes:{type:"tool_calls",',
    )
    return replace_once(
        source,
        'arguments:d(n.arguments??""),files:',
        'arguments:NewApiToolArguments(n.name,n.arguments??""),files:',
    )


def version_static_assets(build):
    static = build / "static"
    custom = build / "studio-v29/custom"
    custom.mkdir(parents=True, exist_ok=True)
    for name in (
        "billing-link.js",
        "studio-group-picker.js",
        "studio-image-picker.js",
        "studio-image-settings.js",
        "studio-image-size-picker.js",
        "studio-image-state.js",
        "studio-link.js",
        "studio-native.js",
        "studio-tool-result.js",
    ):
        path = static / name
        source = path.read_text(encoding="utf-8")
        for previous in (21, 23, 25, 26, 27):
            source = source.replace(f"/studio-v{previous}/", "/studio-v29/").replace(
                f"?v={previous}", "?v=29"
            )
        source = source.replace("/static/studio-", "/studio-v29/custom/studio-")
        path.write_text(source, encoding="utf-8")
        (custom / name).write_text(source, encoding="utf-8")
    shutil.copyfile(static / "billing-bar.css", custom / "billing-bar.css")


def version_native_assets(build):
    # Keep relative module imports together so Svelte and its stores stay singletons.
    native = build / "_app/immutable"
    for path in native.rglob("*.js"):
        source = path.read_text(encoding="utf-8")
        if "/_app/immutable/" in source:
            raise RuntimeError(f"Unexpected absolute native module reference: {path}")
        path.write_text(
            source.replace("?v=23", "?v=29").replace("/static/studio-", "/studio-v29/custom/studio-"),
            encoding="utf-8",
        )
    shutil.copytree(native, build / "studio-v29")
    index = build / "index.html"
    source = index.read_text(encoding="utf-8")
    if "/_app/immutable/" not in source:
        raise RuntimeError("Missing native frontend entry references")
    index.write_text(source.replace("/_app/immutable/", "/studio-v29/"), encoding="utf-8")


def upgrade_frontend(build, sources):
    """Upgrade pinned v23-v27 assets without rebuilding the Open WebUI backend."""
    index = build / "index.html"
    source = index.read_text(encoding="utf-8")
    previous = next((version for version in (27, 26, 25, 23) if f"/studio-v{version}/" in source), None)
    if previous is None:
        raise RuntimeError("Expected the deployed v23, v25, v26 or v27 frontend")
    native = build / f"studio-v{previous}"
    chat = native / "chunks/weExeAJk.js"
    chat_source = chat.read_text(encoding="utf-8")
    patched_chat = patch_chat_preferences(chat_source) if previous == 23 else chat_source
    target = build / "studio-v29"
    if target.exists():
        raise RuntimeError("The v29 asset directory already exists")
    shutil.copytree(native, target)
    (target / "chunks/weExeAJk.js").write_text(patched_chat, encoding="utf-8")
    for path in target.rglob("*.js"):
        source = path.read_text(encoding="utf-8")
        path.write_text(
            source.replace(f"/studio-v{previous}/", "/studio-v29/").replace(
                f"?v={previous}", "?v=29"
            ).replace("/static/studio-", "/studio-v29/custom/studio-"),
            encoding="utf-8",
        )
    for path in sources.glob("*.js"):
        if path.name != "billing-link.js" and not path.name.startswith("studio-"):
            continue
        shutil.copyfile(path, build / "static" / path.name)
    shutil.copyfile(sources / "billing-bar.css", build / "static/billing-bar.css")
    version_static_assets(build)
    source = index.read_text(encoding="utf-8")
    source = source.replace(f"/studio-v{previous}/", "/studio-v29/").replace(
        f"?v={previous}", "?v=29"
    ).replace("?v=24", "?v=29")
    for name in ("billing-link.js", "billing-bar.css", "studio-link.js"):
        source = source.replace(f"/static/{name}", f"/studio-v29/custom/{name}")
    index.write_text(source, encoding="utf-8")


def patch(build):
    node = build / "_app/immutable/nodes/2.mPOUt5NL.js"
    source = node.read_text(encoding="utf-8")
    source = replace_once(
        source,
        'Ee("click",ve,async()=>{const Je=await Gu();',
        've.hidden=true,ve.style.display="none",Ee("click",ve,async()=>{const Je=await Gu();',
    )
    source = replace_once(
        source,
        'Ee("click",De,async()=>{const he=await Gu().catch(fe=>null);',
        'De.hidden=true,De.style.display="none",Ee("click",De,async()=>{const he=await Gu().catch(fe=>null);',
    )
    source = replace_once(
        source,
        'it(S,()=>e(v),Q=>n(v,Q)),i(I,S)',
        'S.readOnly=true,S.dataset.newApiName="",it(S,()=>e(v),Q=>n(v,Q)),i(I,S)',
    )
    source = replace_once(
        source,
        '{id:"admin:connections",titleKey:"settings.admin.connections.title",title:t().t("settings.admin.connections.title"),searchPrefixes:["settings.admin.connections."]},',
        '',
    )
    node.write_text(patch_image_settings(source), encoding="utf-8")

    chat = build / "_app/immutable/chunks/weExeAJk.js"
    chat.write_text(patch_chat(chat.read_text(encoding="utf-8")), encoding="utf-8")

    selector = build / "_app/immutable/chunks/6m72j43W.js"
    selector.write_text(patch_model_selector(selector.read_text(encoding="utf-8")), encoding="utf-8")
    tool_results = build / "_app/immutable/chunks/Cizwf2GR.js"
    tool_results.write_text(patch_tool_results(tool_results.read_text(encoding="utf-8")), encoding="utf-8")
    version_native_assets(build)
    version_static_assets(build)


if __name__ == "__main__":
    patch(Path("/app/build"))
