function OpenModal() {
  $('.large.modal').modal('show');
  if (App.token.length <= 0) {
    GetToken();
  }
}
function CloseModal() {
  $('.large.modal').modal('hide');
}
function GetToken() {
  const data = {action: 'puttoken'};
  $.ajax({
    type:          'POST',
    dataType:      'json',
    contentType:   'application/json',
    scriptCharset: 'utf-8',
    data:          JSON.stringify(data),
    url:           App.url
  })
  .done(function(res) {
    App.token = res.token;
  })
  .fail(function(e) {
    console.log(e);
  });
}
function SetFormValueToken(token) {
  $('#token').attr('value', token);
}
function parseJson (data) {
  var res = {};
  for (i = 0; i < data.length; i++) {
    res[data[i].name] = data[i].value;
  }
  return res;
}
function toBase64 (file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.readAsDataURL(file);
    reader.onload = () => resolve(reader.result);
    reader.onerror = error => reject(error);
  });
}
function onConverted () {
  return function(v) {
    App.imgdata = v;
    $('#preview').attr('src', v);
  }
}
function UploadImage(elm) {
  if (!!App.imgdata) {
    $(elm).addClass("disabled");
    putImage();
  }
}
function putImage() {
  const file = $('#image').prop('files')[0];
  const data = {action: 'uploadimg', filename: file.name, filedata: App.imgdata, token: App.token};
  $.ajax({
    type:          'POST',
    dataType:      'json',
    contentType:   'application/json',
    scriptCharset: 'utf-8',
    data:          JSON.stringify(data),
    url:           App.url
  })
  .fail(function(e) {
    console.log(e);
  })
  .always(function() {
    window.setTimeout(() => location.reload(true), 1000);
  });
}
function ChangeImage () {
  const file = $('#image').prop('files')[0];
  toBase64(file).then(onConverted());
}
var App = { token: '', imgdata: null, url: location.origin + {{ .ApiPath }} };
